package trading_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/lhjnilsson/foreverbull/pkg/finance/internal/suppliers/trading"
	"github.com/lhjnilsson/foreverbull/pkg/finance/supplier"
	pb "github.com/lhjnilsson/foreverbull/pkg/pb/finance"
	"github.com/stretchr/testify/require"
)

const (
	aaplISIN = "US0378331005"
	msftISIN = "US5949181045"

	// alpacaAcceptedBuy is an unfilled equity order. id and status come from the
	// EquityOrderResponse example; qty is the ten-share case.
	// https://docs.alpaca.markets/us/reference/postorder
	alpacaAcceptedBuy = `{
		"id": "7b08df51-c1ac-453c-99f9-323a5f075f0d",
		"symbol": "AAPL",
		"qty": "10",
		"filled_qty": "0",
		"filled_avg_price": null,
		"side": "buy",
		"type": "market",
		"time_in_force": "day",
		"limit_price": null,
		"status": "accepted"
	}`

	// alpacaAccountUSD is GET /v2/account. cash, equity, and currency are strings.
	// portfolio_value is the deprecated alias of equity.
	// https://docs.alpaca.markets/us/reference/getaccount-1
	alpacaAccountUSD = `{
		"cash": "1000.50",
		"equity": "2500.75",
		"portfolio_value": "1",
		"currency": "USD"
	}`

	// alpacaAccountEUR is the same account shape with a non-USD currency.
	// https://docs.alpaca.markets/us/reference/getaccount-1
	alpacaAccountEUR = `{
		"cash": "1000.50",
		"equity": "2500.75",
		"portfolio_value": "2500.75",
		"currency": "EUR"
	}`
)

// alpacaPositionAAPL is one long position. qty is an unsigned share count and
// side is long or short. cost_basis is a string.
// https://docs.alpaca.markets/us/reference/getallopenpositions
const alpacaPositionAAPL = `[{
	"symbol": "AAPL",
	"qty": "5",
	"side": "long",
	"cost_basis": "500.0"
}]`

type recorded struct {
	method string
	path   string
	query  string
	body   []byte
	header http.Header
}

type broker struct {
	mu              sync.Mutex
	calls           []recorded
	accountStatus   int
	accountBody     string
	positionsStatus int
	positionsBody   string
	ordersStatus    int
	ordersBody      string
	placeStatus     int
	placeBody       string
}

func (b *broker) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	raw, _ := io.ReadAll(r.Body)
	b.mu.Lock()
	b.calls = append(b.calls, recorded{
		method: r.Method,
		path:   r.URL.Path,
		query:  r.URL.RawQuery,
		body:   append([]byte(nil), raw...),
		header: r.Header.Clone(),
	})
	status, body := b.response(r)
	b.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

func (b *broker) response(r *http.Request) (int, []byte) {
	switch {
	case r.Method == http.MethodPost && r.URL.Path == "/v2/orders":
		status := b.placeStatus
		if status == 0 {
			status = http.StatusOK
		}
		body := b.placeBody
		if body == "" && status == http.StatusOK {
			body = alpacaAcceptedBuy
		}
		return status, []byte(body)
	case r.Method == http.MethodGet && r.URL.Path == "/v2/account":
		return httpBody(b.accountStatus, b.accountBody)
	case r.Method == http.MethodGet && r.URL.Path == "/v2/positions":
		return httpBody(b.positionsStatus, b.positionsBody)
	case r.Method == http.MethodGet && r.URL.Path == "/v2/orders":
		return httpBody(b.ordersStatus, b.ordersBody)
	default:
		return http.StatusNotFound, []byte(`{"message":"not found"}`)
	}
}

func httpBody(status int, body string) (int, []byte) {
	if status == 0 {
		status = http.StatusOK
	}
	return status, []byte(body)
}

func (b *broker) orderPosts() []recorded {
	b.mu.Lock()
	defer b.mu.Unlock()
	var out []recorded
	for _, call := range b.calls {
		if call.method == http.MethodPost && call.path == "/v2/orders" {
			out = append(out, call)
		}
	}
	return out
}

func newAlpaca(t *testing.T, b *broker) (*trading.AlpacaClient, *trading.Book) {
	t.Helper()
	book, err := trading.Load("")
	require.NoError(t, err)
	return newAlpacaBook(t, b, book), book
}

func newAlpacaBook(t *testing.T, b *broker, book *trading.Book) *trading.AlpacaClient {
	t.Helper()
	if b == nil {
		b = &broker{}
	}
	srv := httptest.NewServer(b)
	t.Cleanup(srv.Close)
	client, err := trading.NewAlpaca(trading.AlpacaConfig{BaseURL: srv.URL}, book, srv.Client())
	require.NoError(t, err)
	return client
}

func aaplRow(t *testing.T, book *trading.Book) trading.Listing {
	t.Helper()
	row, ok := book.Get(aaplISIN)
	require.True(t, ok)
	require.NotEmpty(t, row.AlpacaSymbol)
	return row
}

func marketOrder(amount int32) *pb.Order {
	return &pb.Order{
		Symbol:      "NOT_SENT",
		Isin:        aaplISIN,
		Amount:      amount,
		Type:        pb.OrderType_ORDER_TYPE_MARKET,
		Intent:      pb.PositionIntent_POSITION_INTENT_SELL_TO_OPEN,
		LimitPrice:  15,
		TimeInForce: pb.TimeInForce_TIME_IN_FORCE_DAY,
	}
}

func TestAlpacaMarketBuy(t *testing.T) {
	b := &broker{}
	client, book := newAlpaca(t, b)
	row := aaplRow(t, book)

	got, err := client.PlaceOrder(context.Background(), marketOrder(10))
	require.NoError(t, err)
	require.Equal(t, pb.OrderStatus_ORDER_STATUS_ACCEPTED, got.GetStatus())
	require.Equal(t, "7b08df51-c1ac-453c-99f9-323a5f075f0d", got.GetBrokerOrderId())
	require.Equal(t, row.ISIN, got.GetIsin())
	require.Equal(t, row.Symbol, got.GetSymbol())
	require.Equal(t, int32(10), got.GetAmount())
	require.Equal(t, int32(0), got.GetFilledAmount())

	posts := b.orderPosts()
	require.Len(t, posts, 1)
	// CreateOrderRequest strings. position_intent is not part of this order.
	// https://docs.alpaca.markets/us/reference/postorder
	want := `{"symbol":"` + row.AlpacaSymbol + `","qty":"10","side":"buy","type":"market","time_in_force":"day"}`
	require.Equal(t, want, string(posts[0].body))

	var fields map[string]any
	require.NoError(t, json.Unmarshal(posts[0].body, &fields))
	require.Equal(t, row.AlpacaSymbol, fields["symbol"])
	require.Equal(t, "buy", fields["side"])
	require.Equal(t, "10", fields["qty"])
	require.Equal(t, "market", fields["type"])
	require.Equal(t, "day", fields["time_in_force"])
	_, hasLimit := fields["limit_price"]
	require.False(t, hasLimit)
	_, hasIntent := fields["position_intent"]
	require.False(t, hasIntent)
	require.NotContains(t, string(posts[0].body), "NOT_SENT")
	require.Contains(t, posts[0].header, "Apca-Api-Key-Id")
	require.Contains(t, posts[0].header, "Apca-Api-Secret-Key")
	require.Empty(t, posts[0].header.Get("APCA-API-KEY-ID"))
	require.Empty(t, posts[0].header.Get("APCA-API-SECRET-KEY"))
	require.Equal(t, "application/json", posts[0].header.Get("Content-Type"))
}

func TestAlpacaMarketSell(t *testing.T) {
	b := &broker{placeBody: `{
		"id": "7b08df51-c1ac-453c-99f9-323a5f075f0d",
		"symbol": "AAPL",
		"qty": "10",
		"filled_qty": "0",
		"filled_avg_price": null,
		"side": "sell",
		"type": "market",
		"time_in_force": "day",
		"status": "new"
	}`}
	client, book := newAlpaca(t, b)
	row := aaplRow(t, book)

	got, err := client.PlaceOrder(context.Background(), marketOrder(-10))
	require.NoError(t, err)
	require.Equal(t, int32(-10), got.GetAmount())
	require.Equal(t, pb.OrderStatus_ORDER_STATUS_ACCEPTED, got.GetStatus())
	require.NotEmpty(t, got.GetBrokerOrderId())

	posts := b.orderPosts()
	require.Len(t, posts, 1)
	// Sell is side "sell" with a positive qty string.
	// https://docs.alpaca.markets/us/reference/postorder
	want := `{"symbol":"` + row.AlpacaSymbol + `","qty":"10","side":"sell","type":"market","time_in_force":"day"}`
	require.Equal(t, want, string(posts[0].body))
	require.NotContains(t, string(posts[0].body), "position_intent")
}

func TestAlpacaLimitPrice(t *testing.T) {
	b := &broker{placeBody: `{
		"id": "7b08df51-c1ac-453c-99f9-323a5f075f0d",
		"symbol": "AAPL",
		"qty": "10",
		"filled_qty": "0",
		"filled_avg_price": null,
		"side": "buy",
		"type": "limit",
		"time_in_force": "day",
		"limit_price": "150",
		"status": "accepted"
	}`}
	client, book := newAlpaca(t, b)
	row := aaplRow(t, book)

	order := marketOrder(10)
	order.Type = pb.OrderType_ORDER_TYPE_LIMIT
	order.LimitPrice = 150
	got, err := client.PlaceOrder(context.Background(), order)
	require.NoError(t, err)
	require.Equal(t, pb.OrderType_ORDER_TYPE_LIMIT, got.GetType())
	require.InDelta(t, 150, got.GetLimitPrice(), 1e-9)

	posts := b.orderPosts()
	require.Len(t, posts, 1)
	// limit_price is a string and is required when type is limit.
	// https://docs.alpaca.markets/us/reference/postorder
	want := `{"symbol":"` + row.AlpacaSymbol + `","qty":"10","side":"buy","type":"limit","time_in_force":"day","limit_price":"150"}`
	require.Equal(t, want, string(posts[0].body))

	missing := marketOrder(10)
	missing.Type = pb.OrderType_ORDER_TYPE_LIMIT
	missing.LimitPrice = 0
	placed, err := client.PlaceOrder(context.Background(), missing)
	require.ErrorIs(t, err, supplier.ErrRejected)
	require.Nil(t, placed)

	negative := marketOrder(10)
	negative.Type = pb.OrderType_ORDER_TYPE_LIMIT
	negative.LimitPrice = -1
	placed, err = client.PlaceOrder(context.Background(), negative)
	require.ErrorIs(t, err, supplier.ErrRejected)
	require.Nil(t, placed)
	require.Len(t, b.orderPosts(), 1)
}

func TestAlpacaUnknownISIN(t *testing.T) {
	b := &broker{}
	client, _ := newAlpaca(t, b)
	order := marketOrder(10)
	order.Isin = "US0000000000"
	placed, err := client.PlaceOrder(context.Background(), order)
	require.ErrorIs(t, err, supplier.ErrNotTradable)
	require.Nil(t, placed)
	require.Empty(t, b.orderPosts())
}

func TestAlpacaZeroAmount(t *testing.T) {
	b := &broker{}
	client, _ := newAlpaca(t, b)
	placed, err := client.PlaceOrder(context.Background(), marketOrder(0))
	require.ErrorIs(t, err, supplier.ErrRejected)
	require.Nil(t, placed)
	require.Empty(t, b.orderPosts())
}

func TestAlpacaBrokerHTTP400(t *testing.T) {
	b := &broker{placeStatus: http.StatusBadRequest, placeBody: `{"message":"rejected"}`}
	client, _ := newAlpaca(t, b)
	placed, err := client.PlaceOrder(context.Background(), marketOrder(10))
	require.ErrorIs(t, err, supplier.ErrRejected)
	require.Nil(t, placed)
	require.Len(t, b.orderPosts(), 1)
}

func TestAlpacaBrokerHTTP500(t *testing.T) {
	b := &broker{placeStatus: http.StatusInternalServerError}
	client, _ := newAlpaca(t, b)
	placed, err := client.PlaceOrder(context.Background(), marketOrder(10))
	require.ErrorIs(t, err, supplier.ErrUnavailable)
	require.Nil(t, placed)
	require.Len(t, b.orderPosts(), 1)
}

func TestAlpacaCanceledContext(t *testing.T) {
	b := &broker{}
	client, _ := newAlpaca(t, b)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	placed, err := client.PlaceOrder(ctx, marketOrder(10))
	require.ErrorIs(t, err, context.Canceled)
	require.Nil(t, placed)
	require.Empty(t, b.orderPosts())
}

func TestAlpacaGetPortfolio(t *testing.T) {
	b := &broker{accountBody: alpacaAccountUSD, positionsBody: alpacaPositionAAPL}
	client, book := newAlpaca(t, b)
	row := aaplRow(t, book)

	got, err := client.GetPortfolio(context.Background())
	require.NoError(t, err)
	require.NotNil(t, got.GetTimestamp())
	require.InDelta(t, 1000.50, got.GetCash(), 1e-9)
	require.InDelta(t, 2500.75, got.GetPortfolioValue(), 1e-9)
	require.Len(t, got.GetPositions(), 1)
	pos := got.GetPositions()[0]
	require.Equal(t, row.ISIN, pos.GetIsin())
	require.Equal(t, row.Symbol, pos.GetSymbol())
	require.Equal(t, int32(5), pos.GetAmount())
	require.InDelta(t, 500, pos.GetCostBasis(), 1e-9)
	require.Empty(t, b.orderPosts())
}

func TestAlpacaUnmappedPosition(t *testing.T) {
	b := &broker{
		accountBody: alpacaAccountUSD,
		// Symbol is not an alpaca_symbol in the sample listing.
		// https://docs.alpaca.markets/us/reference/getallopenpositions
		positionsBody: `[{"symbol":"ZZZZ","qty":"1","side":"long","cost_basis":"1"}]`,
	}
	client, _ := newAlpaca(t, b)
	got, err := client.GetPortfolio(context.Background())
	require.ErrorIs(t, err, supplier.ErrUnmappedPosition)
	require.Nil(t, got)
}

func TestAlpacaCurrencyMismatch(t *testing.T) {
	b := &broker{accountBody: alpacaAccountEUR, positionsBody: alpacaPositionAAPL}
	client, _ := newAlpaca(t, b)
	got, err := client.GetPortfolio(context.Background())
	require.ErrorIs(t, err, supplier.ErrCurrency)
	require.Nil(t, got)
}

func TestAlpacaGetOrders(t *testing.T) {
	b := &broker{}
	client, book := newAlpaca(t, b)
	row := aaplRow(t, book)
	// GET /v2/orders returns an array. Defaults to open; id is the broker order id.
	// https://docs.alpaca.markets/us/reference/getallorders-1
	b.ordersBody = `[{
		"id": "7b08df51-c1ac-453c-99f9-323a5f075f0d",
		"symbol": "` + row.AlpacaSymbol + `",
		"qty": "2",
		"filled_qty": "0",
		"filled_avg_price": null,
		"side": "buy",
		"type": "limit",
		"time_in_force": "day",
		"limit_price": "150",
		"status": "accepted"
	}]`

	got, err := client.GetOrders(context.Background())
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, "7b08df51-c1ac-453c-99f9-323a5f075f0d", got[0].GetBrokerOrderId())
	require.Equal(t, row.ISIN, got[0].GetIsin())
	require.Equal(t, row.Symbol, got[0].GetSymbol())
	require.Equal(t, int32(2), got[0].GetAmount())
	require.Equal(t, pb.OrderStatus_ORDER_STATUS_ACCEPTED, got[0].GetStatus())
	require.Equal(t, pb.OrderType_ORDER_TYPE_LIMIT, got[0].GetType())
	require.InDelta(t, 150, got[0].GetLimitPrice(), 1e-9)

	require.NotEmpty(t, b.calls)
	var sawOpen bool
	for _, call := range b.calls {
		if call.method == http.MethodGet && call.path == "/v2/orders" {
			sawOpen = true
			require.Contains(t, call.query, "status=open")
		}
	}
	require.True(t, sawOpen)

	emptyBroker := &broker{ordersBody: `[]`}
	emptyClient, _ := newAlpaca(t, emptyBroker)
	none, err := emptyClient.GetOrders(context.Background())
	require.NoError(t, err)
	require.Empty(t, none)
}

func TestAlpacaLimitOrderZeroOrMissingPrice(t *testing.T) {
	b := &broker{}
	client, book := newAlpaca(t, b)
	row := aaplRow(t, book)
	// limit_price is a string. Zero, negative, and omitted are not a price.
	// https://docs.alpaca.markets/us/reference/getallorders-1
	order := func(limit string) string {
		return `[{
			"id": "7b08df51-c1ac-453c-99f9-323a5f075f0d",
			"symbol": "` + row.AlpacaSymbol + `",
			"qty": "2",
			"filled_qty": "0",
			"filled_avg_price": null,
			"side": "buy",
			"type": "limit",
			"time_in_force": "day",
			` + limit + `
			"status": "accepted"
		}]`
	}
	bodies := []string{
		order(`"limit_price": "0",`),
		order(`"limit_price": "-1",`),
		order(`"limit_price": "",`),
		order(`"limit_price": " ",`),
		order(`"limit_price": null,`),
		order(""),
	}
	for _, body := range bodies {
		b.ordersBody = body
		got, err := client.GetOrders(context.Background())
		require.ErrorIs(t, err, supplier.ErrUnavailable)
		require.Nil(t, got)
	}
}

func TestAlpacaEmptyISIN(t *testing.T) {
	b := &broker{}
	client, _ := newAlpaca(t, b)
	order := marketOrder(10)
	order.Isin = "  "
	placed, err := client.PlaceOrder(context.Background(), order)
	require.ErrorIs(t, err, supplier.ErrNotTradable)
	require.Nil(t, placed)
	require.Empty(t, b.orderPosts())
}

func TestAlpacaEmptyBrokerSymbol(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "listings.json")
	raw := []byte(`[{
		"isin": "US0378331005",
		"symbol": "AAPL",
		"currency": "USD",
		"alpaca_symbol": "",
		"saxo_uic": 0,
		"saxo_asset_type": "",
		"ibkr_conid": 0,
		"ig_epic": "",
		"ig_share_size": 0
	}]`)
	require.NoError(t, os.WriteFile(path, raw, 0o644))
	book, err := trading.Load(path)
	require.NoError(t, err)

	b := &broker{}
	client := newAlpacaBook(t, b, book)
	placed, err := client.PlaceOrder(context.Background(), marketOrder(10))
	require.ErrorIs(t, err, supplier.ErrNotTradable)
	require.Nil(t, placed)
	require.Empty(t, b.orderPosts())
}

func TestAlpacaPartialFill(t *testing.T) {
	b := &broker{placeBody: `{
		"id": "7b08df51-c1ac-453c-99f9-323a5f075f0d",
		"symbol": "AAPL",
		"qty": "10",
		"filled_qty": "4",
		"filled_avg_price": "101.5",
		"side": "sell",
		"type": "market",
		"time_in_force": "day",
		"status": "partially_filled"
	}`}
	client, _ := newAlpaca(t, b)
	got, err := client.PlaceOrder(context.Background(), marketOrder(-10))
	require.NoError(t, err)
	require.Equal(t, pb.OrderStatus_ORDER_STATUS_PARTIALLY_FILLED, got.GetStatus())
	require.Equal(t, int32(-4), got.GetFilledAmount())
	require.InDelta(t, 101.5, got.GetAveragePrice(), 1e-9)

	b.placeBody = `{
		"id": "7b08df51-c1ac-453c-99f9-323a5f075f0d",
		"symbol": "AAPL",
		"qty": "10",
		"filled_qty": "10",
		"filled_avg_price": "101.5",
		"side": "buy",
		"type": "market",
		"time_in_force": "day",
		"status": "filled"
	}`
	filled, err := client.PlaceOrder(context.Background(), marketOrder(10))
	require.NoError(t, err)
	require.Equal(t, pb.OrderStatus_ORDER_STATUS_FILLED, filled.GetStatus())
	require.Equal(t, int32(10), filled.GetFilledAmount())
}

func TestAlpacaBrokerHTTP401(t *testing.T) {
	b := &broker{placeStatus: http.StatusUnauthorized}
	client, _ := newAlpaca(t, b)
	placed, err := client.PlaceOrder(context.Background(), marketOrder(10))
	require.ErrorIs(t, err, supplier.ErrUnavailable)
	require.Nil(t, placed)
}

func TestAlpacaLocalRejects(t *testing.T) {
	b := &broker{}
	client, _ := newAlpaca(t, b)

	badTIF := marketOrder(10)
	badTIF.TimeInForce = pb.TimeInForce(99)
	placed, err := client.PlaceOrder(context.Background(), badTIF)
	require.ErrorIs(t, err, supplier.ErrRejected)
	require.Nil(t, placed)

	badType := marketOrder(10)
	badType.Type = pb.OrderType_ORDER_TYPE_UNSPECIFIED
	placed, err = client.PlaceOrder(context.Background(), badType)
	require.ErrorIs(t, err, supplier.ErrRejected)
	require.Nil(t, placed)
	require.Empty(t, b.orderPosts())
}

func TestAlpacaShortPositionIsNegative(t *testing.T) {
	b := &broker{
		accountBody: alpacaAccountUSD,
		// https://docs.alpaca.markets/us/reference/getallopenpositions
		positionsBody: `[{"symbol":"MSFT","qty":"3","side":"short","cost_basis":"90"}]`,
	}
	client, book := newAlpaca(t, b)
	row, ok := book.Get(msftISIN)
	require.True(t, ok)

	got, err := client.GetPortfolio(context.Background())
	require.NoError(t, err)
	require.Len(t, got.GetPositions(), 1)
	require.Equal(t, row.ISIN, got.GetPositions()[0].GetIsin())
	require.Equal(t, row.Symbol, got.GetPositions()[0].GetSymbol())
	require.Equal(t, int32(-3), got.GetPositions()[0].GetAmount())
}

func TestAlpacaPaperHostDefault(t *testing.T) {
	book, err := trading.Load("")
	require.NoError(t, err)
	transport := &captureTransport{}
	client, err := trading.NewAlpaca(trading.AlpacaConfig{}, book, &http.Client{Transport: transport})
	require.NoError(t, err)
	_, err = client.GetPortfolio(context.Background())
	require.ErrorIs(t, err, supplier.ErrUnavailable)
	require.NotNil(t, transport.req)
	require.Equal(t, "paper-api.alpaca.markets", transport.req.URL.Host)
	require.Equal(t, "/v2/account", transport.req.URL.Path)
	require.Equal(t, "https", transport.req.URL.Scheme)
}

func TestAlpacaLiveHostRefusedUnlessAllowed(t *testing.T) {
	book, err := trading.Load("")
	require.NoError(t, err)
	_, err = trading.NewAlpaca(trading.AlpacaConfig{
		BaseURL: "https://api.alpaca.markets",
	}, book, &http.Client{})
	require.Error(t, err)

	_, err = trading.NewAlpaca(trading.AlpacaConfig{
		BaseURL:      "https://api.alpaca.markets",
		AllowLiveURL: true,
	}, book, &http.Client{})
	require.NoError(t, err)

	_, err = trading.NewAlpaca(trading.AlpacaConfig{
		BaseURL: "https://paper-api.alpaca.markets",
	}, book, &http.Client{})
	require.NoError(t, err)
}

type captureTransport struct {
	req *http.Request
}

func (c *captureTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	c.req = r
	return nil, errors.New("stopped")
}
