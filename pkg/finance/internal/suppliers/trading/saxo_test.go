package trading_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/lhjnilsson/foreverbull/pkg/finance/internal/suppliers/trading"
	"github.com/lhjnilsson/foreverbull/pkg/finance/supplier"
	pb "github.com/lhjnilsson/foreverbull/pkg/pb/finance"
	"github.com/stretchr/testify/require"
)

const (
	// saxoAcceptedBuy is a placed order. OrderId is a string.
	// https://www.developer.saxo/openapi/learn/order-placement-return-codes
	saxoAcceptedBuy = `{"OrderId":"74993782"}`

	// saxoAccountUSD is one simulation account. Currency is the account currency.
	// https://www.developer.saxo/openapi/referencedocs/port/v1/accounts/get__port__me
	saxoAccountUSD = `{"Data":[{"Currency":"USD"}]}`

	// saxoAccountEUR is the same account shape with a non-USD currency.
	// https://www.developer.saxo/openapi/referencedocs/port/v1/accounts/get__port__accountkey
	saxoAccountEUR = `{"Data":[{"Currency":"EUR"}]}`

	// saxoBalanceUSD is GET /port/v1/balances/me. CashBalance is cash.
	// TotalValue is the current portfolio value. Currency denominates both.
	// https://www.developer.saxo/openapi/referencedocs/port/v1/balances/get__port__me
	saxoBalanceUSD = `{
		"CashBalance": 1000.50,
		"TotalValue": 2500.75,
		"Currency": "USD"
	}`
)

// saxoPositionAAPL is one net position. Amount is the net share volume.
// Uic plus AssetType identify the instrument. MarketValueOpen is the open value.
// https://www.developer.saxo/openapi/referencedocs/port/v1/netpositions/get__port__me
// https://www.developer.saxo/openapi/referencedocs/port/v1/netpositions/get__port__netpositionid/schema-netpositiondynamic
const saxoPositionAAPL = `{
	"Data": [{
		"NetPositionBase": {"Amount": 5, "AssetType": "Stock", "Uic": 211},
		"NetPositionView": {"MarketValueOpen": 500}
	}]
}`

type saxoCall struct {
	method string
	path   string
	query  string
	body   []byte
	header http.Header
}

type saxoBroker struct {
	mu              sync.Mutex
	calls           []saxoCall
	accountStatus   int
	accountBody     string
	balanceStatus   int
	balanceBody     string
	positionsStatus int
	positionsBody   string
	ordersStatus    int
	ordersBody      string
	placeStatus     int
	placeBody       string
}

func (b *saxoBroker) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	raw, _ := io.ReadAll(r.Body)
	b.mu.Lock()
	b.calls = append(b.calls, saxoCall{
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

func (b *saxoBroker) response(r *http.Request) (int, []byte) {
	switch {
	case r.Method == http.MethodPost && r.URL.Path == "/sim/openapi/trade/v2/orders":
		status := b.placeStatus
		if status == 0 {
			status = http.StatusOK
		}
		body := b.placeBody
		if body == "" && status == http.StatusOK {
			body = saxoAcceptedBuy
		}
		return status, []byte(body)
	case r.Method == http.MethodGet && r.URL.Path == "/sim/openapi/port/v1/accounts/me":
		return httpBody(b.accountStatus, b.accountBody)
	case r.Method == http.MethodGet && r.URL.Path == "/sim/openapi/port/v1/balances/me":
		return httpBody(b.balanceStatus, b.balanceBody)
	case r.Method == http.MethodGet && r.URL.Path == "/sim/openapi/port/v1/netpositions/me":
		return httpBody(b.positionsStatus, b.positionsBody)
	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/sim/openapi/port/v1/orders/me"):
		return httpBody(b.ordersStatus, b.ordersBody)
	default:
		return http.StatusNotFound, []byte(`{"Message":"not found"}`)
	}
}

func (b *saxoBroker) orderPosts() []saxoCall {
	b.mu.Lock()
	defer b.mu.Unlock()
	var out []saxoCall
	for _, call := range b.calls {
		if call.method == http.MethodPost && call.path == "/sim/openapi/trade/v2/orders" {
			out = append(out, call)
		}
	}
	return out
}

func newSaxo(t *testing.T, b *saxoBroker) (*trading.SaxoClient, *trading.Book) {
	t.Helper()
	book, err := trading.Load("")
	require.NoError(t, err)
	return newSaxoBook(t, b, book, trading.SaxoConfig{}), book
}

func newSaxoBook(t *testing.T, b *saxoBroker, book *trading.Book, cfg trading.SaxoConfig) *trading.SaxoClient {
	t.Helper()
	if b == nil {
		b = &saxoBroker{}
	}
	srv := httptest.NewServer(b)
	t.Cleanup(srv.Close)
	cfg.BaseURL = srv.URL + "/sim/openapi"
	client, err := trading.NewSaxo(cfg, book, srv.Client())
	require.NoError(t, err)
	return client
}

func aaplSaxo(t *testing.T, book *trading.Book) trading.Listing {
	t.Helper()
	row, ok := book.Get(aaplISIN)
	require.True(t, ok)
	require.NotZero(t, row.SaxoUIC)
	require.NotEmpty(t, row.SaxoAssetType)
	return row
}

func saxoMarketBody(row trading.Listing, side string, qty int32) string {
	return fmt.Sprintf(
		`{"Amount":%d,"BuySell":%q,"OrderType":"Market","ManualOrder":false,"Uic":%d,"AssetType":%q,"OrderDuration":{"DurationType":"DayOrder"}}`,
		qty, side, row.SaxoUIC, row.SaxoAssetType,
	)
}

func TestSaxoMarketBuy(t *testing.T) {
	b := &saxoBroker{}
	client, book := newSaxo(t, b)
	row := aaplSaxo(t, book)

	got, err := client.PlaceOrder(context.Background(), marketOrder(10))
	require.NoError(t, err)
	require.Equal(t, pb.OrderStatus_ORDER_STATUS_ACCEPTED, got.GetStatus())
	require.Equal(t, "74993782", got.GetBrokerOrderId())
	require.Equal(t, row.ISIN, got.GetIsin())
	require.Equal(t, row.Symbol, got.GetSymbol())
	require.Equal(t, int32(10), got.GetAmount())
	require.Equal(t, int32(0), got.GetFilledAmount())

	posts := b.orderPosts()
	require.Len(t, posts, 1)
	// BuySell, Amount, Uic, AssetType, and OrderType Market. DayOrder is the day duration.
	// ManualOrder false is an automatic strategy order. AccountKey is omitted when unset.
	// https://www.developer.saxo/openapi/learn/order-placement
	require.Equal(t, saxoMarketBody(row, "Buy", 10), string(posts[0].body))

	var fields map[string]any
	require.NoError(t, json.Unmarshal(posts[0].body, &fields))
	require.InDelta(t, float64(row.SaxoUIC), fields["Uic"], 0)
	require.Equal(t, row.SaxoAssetType, fields["AssetType"])
	require.Equal(t, "Buy", fields["BuySell"])
	require.InDelta(t, 10, fields["Amount"], 0)
	require.Equal(t, "Market", fields["OrderType"])
	_, hasPrice := fields["OrderPrice"]
	require.False(t, hasPrice)
	_, hasOpen := fields["ToOpenClose"]
	require.False(t, hasOpen)
	require.NotContains(t, string(posts[0].body), "NOT_SENT")
	require.Equal(t, "Bearer", posts[0].header.Get("Authorization"))
	require.Equal(t, "application/json", posts[0].header.Get("Content-Type"))
}

func TestSaxoMarketSell(t *testing.T) {
	b := &saxoBroker{}
	client, book := newSaxo(t, b)
	row := aaplSaxo(t, book)

	got, err := client.PlaceOrder(context.Background(), marketOrder(-10))
	require.NoError(t, err)
	require.Equal(t, int32(-10), got.GetAmount())
	require.Equal(t, pb.OrderStatus_ORDER_STATUS_ACCEPTED, got.GetStatus())
	require.NotEmpty(t, got.GetBrokerOrderId())

	posts := b.orderPosts()
	require.Len(t, posts, 1)
	// Sell is BuySell "Sell" with a positive Amount.
	// https://www.developer.saxo/openapi/learn/order-placement
	require.Equal(t, saxoMarketBody(row, "Sell", 10), string(posts[0].body))
	require.NotContains(t, string(posts[0].body), "ToOpenClose")
}

func TestSaxoLimitPrice(t *testing.T) {
	b := &saxoBroker{}
	client, book := newSaxo(t, b)
	row := aaplSaxo(t, book)

	order := marketOrder(10)
	order.Type = pb.OrderType_ORDER_TYPE_LIMIT
	order.LimitPrice = 150
	got, err := client.PlaceOrder(context.Background(), order)
	require.NoError(t, err)
	require.Equal(t, pb.OrderType_ORDER_TYPE_LIMIT, got.GetType())
	require.InDelta(t, 150, got.GetLimitPrice(), 1e-9)

	posts := b.orderPosts()
	require.Len(t, posts, 1)
	// OrderPrice is required for Limit and omitted for Market.
	// https://www.developer.saxo/openapi/learn/order-placement
	want := fmt.Sprintf(
		`{"Amount":10,"BuySell":"Buy","OrderType":"Limit","OrderPrice":150,"ManualOrder":false,"Uic":%d,"AssetType":%q,"OrderDuration":{"DurationType":"DayOrder"}}`,
		row.SaxoUIC, row.SaxoAssetType,
	)
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

func TestSaxoUnknownISIN(t *testing.T) {
	b := &saxoBroker{}
	client, _ := newSaxo(t, b)
	order := marketOrder(10)
	order.Isin = "US0000000000"
	placed, err := client.PlaceOrder(context.Background(), order)
	require.ErrorIs(t, err, supplier.ErrNotTradable)
	require.Nil(t, placed)
	require.Empty(t, b.orderPosts())
}

func TestSaxoZeroAmount(t *testing.T) {
	b := &saxoBroker{}
	client, _ := newSaxo(t, b)
	placed, err := client.PlaceOrder(context.Background(), marketOrder(0))
	require.ErrorIs(t, err, supplier.ErrRejected)
	require.Nil(t, placed)
	require.Empty(t, b.orderPosts())
}

func TestSaxoBrokerHTTP400(t *testing.T) {
	// ErrorCode and Message are the 400 body.
	// https://www.developer.saxo/openapi/learn/openapi-request-response
	b := &saxoBroker{
		placeStatus: http.StatusBadRequest,
		placeBody:   `{"ErrorCode":"InvalidModelState","Message":"One or more properties of the request are invalid!"}`,
	}
	client, _ := newSaxo(t, b)
	placed, err := client.PlaceOrder(context.Background(), marketOrder(10))
	require.ErrorIs(t, err, supplier.ErrRejected)
	require.Nil(t, placed)
	require.Len(t, b.orderPosts(), 1)
}

func TestSaxoBrokerHTTP500(t *testing.T) {
	b := &saxoBroker{placeStatus: http.StatusInternalServerError}
	client, _ := newSaxo(t, b)
	placed, err := client.PlaceOrder(context.Background(), marketOrder(10))
	require.ErrorIs(t, err, supplier.ErrUnavailable)
	require.Nil(t, placed)
	require.Len(t, b.orderPosts(), 1)
}

func TestSaxoCanceledContext(t *testing.T) {
	b := &saxoBroker{}
	client, _ := newSaxo(t, b)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	placed, err := client.PlaceOrder(ctx, marketOrder(10))
	require.ErrorIs(t, err, context.Canceled)
	require.Nil(t, placed)
	require.Empty(t, b.orderPosts())
	require.Empty(t, b.calls)
}

func TestSaxoGetPortfolio(t *testing.T) {
	b := &saxoBroker{
		accountBody:   saxoAccountUSD,
		balanceBody:   saxoBalanceUSD,
		positionsBody: saxoPositionAAPL,
	}
	client, book := newSaxo(t, b)
	row := aaplSaxo(t, book)

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

func TestSaxoUnmappedPosition(t *testing.T) {
	b := &saxoBroker{
		accountBody: saxoAccountUSD,
		balanceBody: saxoBalanceUSD,
		// Uic is not a saxo_uic in the sample listing.
		// https://www.developer.saxo/openapi/referencedocs/port/v1/netpositions/get__port__me
		positionsBody: `{"Data":[{"NetPositionBase":{"Amount":1,"AssetType":"Stock","Uic":999999}}]}`,
	}
	client, _ := newSaxo(t, b)
	got, err := client.GetPortfolio(context.Background())
	require.ErrorIs(t, err, supplier.ErrUnmappedPosition)
	require.Nil(t, got)
}

func TestSaxoCurrencyMismatch(t *testing.T) {
	b := &saxoBroker{accountBody: saxoAccountEUR, balanceBody: saxoBalanceUSD, positionsBody: saxoPositionAAPL}
	client, _ := newSaxo(t, b)
	got, err := client.GetPortfolio(context.Background())
	require.ErrorIs(t, err, supplier.ErrCurrency)
	require.Nil(t, got)
}

func TestSaxoGetOrders(t *testing.T) {
	b := &saxoBroker{}
	client, book := newSaxo(t, b)
	row := aaplSaxo(t, book)
	// GET /port/v1/orders/me returns Data. OrderId is the broker order id.
	// OpenOrderType and Price are the working limit. Status Working is open.
	// https://www.developer.saxo/openapi/referencedocs/port/v1/orders/get__port__me
	b.ordersBody = fmt.Sprintf(`{"Data":[{
		"Amount": 2,
		"AssetType": %q,
		"BuySell": "Buy",
		"Duration": {"DurationType": "DayOrder"},
		"FilledAmount": 0,
		"OpenOrderType": "Limit",
		"OrderId": "49318458",
		"Price": 150,
		"Status": "Working",
		"Uic": %d
	}]}`, row.SaxoAssetType, row.SaxoUIC)

	got, err := client.GetOrders(context.Background())
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, "49318458", got[0].GetBrokerOrderId())
	require.Equal(t, row.ISIN, got[0].GetIsin())
	require.Equal(t, row.Symbol, got[0].GetSymbol())
	require.Equal(t, int32(2), got[0].GetAmount())
	require.Equal(t, pb.OrderStatus_ORDER_STATUS_ACCEPTED, got[0].GetStatus())
	require.Equal(t, pb.OrderType_ORDER_TYPE_LIMIT, got[0].GetType())
	require.Equal(t, pb.TimeInForce_TIME_IN_FORCE_DAY, got[0].GetTimeInForce())
	require.InDelta(t, 150, got[0].GetLimitPrice(), 1e-9)

	require.NotEmpty(t, b.calls)
	var sawOpen bool
	for _, call := range b.calls {
		if call.method == http.MethodGet && call.path == "/sim/openapi/port/v1/orders/me" {
			sawOpen = true
			require.Contains(t, call.query, "Status=Working")
		}
	}
	require.True(t, sawOpen)

	emptyBroker := &saxoBroker{ordersBody: `{"Data":[]}`}
	emptyClient, _ := newSaxo(t, emptyBroker)
	none, err := emptyClient.GetOrders(context.Background())
	require.NoError(t, err)
	require.Empty(t, none)
}

func TestSaxoEmptyISIN(t *testing.T) {
	b := &saxoBroker{}
	client, _ := newSaxo(t, b)
	order := marketOrder(10)
	order.Isin = "  "
	placed, err := client.PlaceOrder(context.Background(), order)
	require.ErrorIs(t, err, supplier.ErrNotTradable)
	require.Nil(t, placed)
	require.Empty(t, b.orderPosts())
}

func TestSaxoEmptyBrokerID(t *testing.T) {
	book, err := trading.Load("")
	require.NoError(t, err)
	row, ok := book.Get("US1912161007")
	require.True(t, ok)
	require.Zero(t, row.SaxoUIC)

	b := &saxoBroker{}
	client, _ := newSaxo(t, b)
	order := marketOrder(10)
	order.Isin = row.ISIN
	placed, err := client.PlaceOrder(context.Background(), order)
	require.ErrorIs(t, err, supplier.ErrNotTradable)
	require.Nil(t, placed)
	require.Empty(t, b.orderPosts())
}

func TestSaxoListingUIC(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "listings.json")
	raw := []byte(`[{
		"isin": "US0378331005",
		"symbol": "AAPL",
		"currency": "USD",
		"alpaca_symbol": "AAPL",
		"saxo_uic": 424242,
		"saxo_asset_type": "Stock",
		"ibkr_conid": 0,
		"ig_epic": "",
		"ig_share_size": 0
	}]`)
	require.NoError(t, os.WriteFile(path, raw, 0o644))
	book, err := trading.Load(path)
	require.NoError(t, err)

	b := &saxoBroker{}
	client := newSaxoBook(t, b, book, trading.SaxoConfig{})
	_, err = client.PlaceOrder(context.Background(), marketOrder(10))
	require.NoError(t, err)
	posts := b.orderPosts()
	require.Len(t, posts, 1)
	require.Contains(t, string(posts[0].body), `"Uic":424242`)
	require.NotContains(t, string(posts[0].body), `"Uic":211`)
}

func TestSaxoPartialFill(t *testing.T) {
	b := &saxoBroker{}
	client, book := newSaxo(t, b)
	row := aaplSaxo(t, book)
	// FilledAmount is the amount already filled. AveragePrice is that fill.
	// https://www.developer.saxo/openapi/referencedocs/port/v1/orders/get__port/schema-orderresponse
	// https://www.developer.saxo/openapi/learn/order-events
	b.ordersBody = fmt.Sprintf(`{"Data":[{
		"Amount": 10,
		"AssetType": %q,
		"AveragePrice": 99.9,
		"BuySell": "Sell",
		"FilledAmount": 4,
		"OpenOrderType": "Market",
		"OrderId": "49318458",
		"Uic": %d
	}]}`, row.SaxoAssetType, row.SaxoUIC)

	got, err := client.GetOrders(context.Background())
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, pb.OrderStatus_ORDER_STATUS_PARTIALLY_FILLED, got[0].GetStatus())
	require.Equal(t, int32(-10), got[0].GetAmount())
	require.Equal(t, int32(-4), got[0].GetFilledAmount())
	require.InDelta(t, 99.9, got[0].GetAveragePrice(), 1e-9)
}

func TestSaxoFilledOrderMissingAveragePrice(t *testing.T) {
	b := &saxoBroker{}
	client, book := newSaxo(t, b)
	row := aaplSaxo(t, book)
	// A fill with no AveragePrice is not stored as price 0.
	// https://www.developer.saxo/openapi/learn/order-events
	b.ordersBody = fmt.Sprintf(`{"Data":[{
		"Amount": 10,
		"AssetType": %q,
		"BuySell": "Sell",
		"FilledAmount": 4,
		"OpenOrderType": "Market",
		"OrderId": "49318458",
		"Uic": %d
	}]}`, row.SaxoAssetType, row.SaxoUIC)

	got, err := client.GetOrders(context.Background())
	require.ErrorIs(t, err, supplier.ErrUnavailable)
	require.Nil(t, got)
}

func TestSaxoOpenOrderZeroAmountOrLimit(t *testing.T) {
	b := &saxoBroker{}
	client, book := newSaxo(t, b)
	row := aaplSaxo(t, book)
	// Amount 0 is not a size. A limit Price of 0 or below is not a trigger.
	// https://www.developer.saxo/openapi/referencedocs/port/v1/orders/get__port/schema-orderresponse
	bodies := []string{
		fmt.Sprintf(`{"Data":[{
			"Amount": 0,
			"AssetType": %q,
			"BuySell": "Buy",
			"FilledAmount": 0,
			"OpenOrderType": "Market",
			"OrderId": "49318458",
			"Uic": %d
		}]}`, row.SaxoAssetType, row.SaxoUIC),
		fmt.Sprintf(`{"Data":[{
			"Amount": 2,
			"AssetType": %q,
			"BuySell": "Buy",
			"FilledAmount": 0,
			"OpenOrderType": "Limit",
			"OrderId": "49318458",
			"Price": 0,
			"Uic": %d
		}]}`, row.SaxoAssetType, row.SaxoUIC),
		fmt.Sprintf(`{"Data":[{
			"Amount": 2,
			"AssetType": %q,
			"BuySell": "Buy",
			"FilledAmount": 0,
			"OpenOrderType": "Limit",
			"OrderId": "49318458",
			"Price": -1,
			"Uic": %d
		}]}`, row.SaxoAssetType, row.SaxoUIC),
		fmt.Sprintf(`{"Data":[{
			"Amount": 2,
			"AssetType": %q,
			"BuySell": "Buy",
			"FilledAmount": 0,
			"OpenOrderType": "Limit",
			"OrderId": "49318458",
			"Uic": %d
		}]}`, row.SaxoAssetType, row.SaxoUIC),
	}
	for _, body := range bodies {
		b.ordersBody = body
		got, err := client.GetOrders(context.Background())
		require.ErrorIs(t, err, supplier.ErrUnavailable)
		require.Nil(t, got)
	}
}

func TestSaxoBrokerHTTP401(t *testing.T) {
	b := &saxoBroker{placeStatus: http.StatusUnauthorized}
	client, _ := newSaxo(t, b)
	placed, err := client.PlaceOrder(context.Background(), marketOrder(10))
	require.ErrorIs(t, err, supplier.ErrUnavailable)
	require.Nil(t, placed)
}

func TestSaxoLocalRejects(t *testing.T) {
	b := &saxoBroker{}
	client, _ := newSaxo(t, b)

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

func TestSaxoShortPositionIsNegative(t *testing.T) {
	b := &saxoBroker{
		accountBody: saxoAccountUSD,
		balanceBody: saxoBalanceUSD,
		// Net Amount is signed. Opposing trades net to zero, so a short is negative.
		// https://www.developer.saxo/openapi/learn/fifo-end-of-day-netting
		positionsBody: `{"Data":[{"NetPositionBase":{"Amount":-3,"AssetType":"Stock","Uic":261}}]}`,
	}
	client, book := newSaxo(t, b)
	row, ok := book.Get(msftISIN)
	require.True(t, ok)

	got, err := client.GetPortfolio(context.Background())
	require.NoError(t, err)
	require.Len(t, got.GetPositions(), 1)
	require.Equal(t, row.ISIN, got.GetPositions()[0].GetIsin())
	require.Equal(t, row.Symbol, got.GetPositions()[0].GetSymbol())
	require.Equal(t, int32(-3), got.GetPositions()[0].GetAmount())
}

func TestSaxoPlaceErrorInfo(t *testing.T) {
	// HTTP 200 still fails when ErrorInfo is set. OrderId alone is not success.
	// https://www.developer.saxo/openapi/referencedocs/trade/v2/orders/post__trade
	b := &saxoBroker{placeBody: `{
		"ErrorInfo": {"ErrorCode": "IllegalAmount", "Message": "Illegal amount"},
		"OrderId": "74993782"
	}`}
	client, _ := newSaxo(t, b)
	placed, err := client.PlaceOrder(context.Background(), marketOrder(10))
	require.ErrorIs(t, err, supplier.ErrRejected)
	require.Nil(t, placed)
}

func TestSaxoSimHostDefault(t *testing.T) {
	book, err := trading.Load("")
	require.NoError(t, err)
	transport := &captureTransport{}
	client, err := trading.NewSaxo(trading.SaxoConfig{}, book, &http.Client{Transport: transport})
	require.NoError(t, err)
	_, err = client.GetPortfolio(context.Background())
	require.ErrorIs(t, err, supplier.ErrUnavailable)
	require.NotNil(t, transport.req)
	require.Equal(t, "gateway.saxobank.com", transport.req.URL.Host)
	require.Equal(t, "/sim/openapi/port/v1/accounts/me", transport.req.URL.Path)
	require.Equal(t, "https", transport.req.URL.Scheme)
	require.Equal(t, "Bearer", transport.req.Header.Get("Authorization"))
}

func TestSaxoLiveURLRefusedUnlessAllowed(t *testing.T) {
	book, err := trading.Load("")
	require.NoError(t, err)
	_, err = trading.NewSaxo(trading.SaxoConfig{
		BaseURL: "https://gateway.saxobank.com/openapi",
	}, book, &http.Client{})
	require.Error(t, err)

	_, err = trading.NewSaxo(trading.SaxoConfig{
		BaseURL:      "https://gateway.saxobank.com/openapi",
		AllowLiveURL: true,
	}, book, &http.Client{})
	require.NoError(t, err)

	_, err = trading.NewSaxo(trading.SaxoConfig{
		BaseURL: "https://gateway.saxobank.com/sim/openapi",
	}, book, &http.Client{})
	require.NoError(t, err)

	_, err = trading.NewSaxo(trading.SaxoConfig{
		BaseURL: "https://gateway.saxobank.com/sim/../openapi",
	}, book, &http.Client{})
	require.Error(t, err)
}

func TestSaxoNumericOrderID(t *testing.T) {
	// The placement article also shows OrderId as a JSON number.
	// https://www.developer.saxo/openapi/learn/order-placement
	b := &saxoBroker{placeBody: `{"OrderId":49036233}`}
	client, _ := newSaxo(t, b)
	got, err := client.PlaceOrder(context.Background(), marketOrder(10))
	require.NoError(t, err)
	require.Equal(t, "49036233", got.GetBrokerOrderId())
	require.Equal(t, pb.OrderStatus_ORDER_STATUS_ACCEPTED, got.GetStatus())
}
