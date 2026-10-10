package trading_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
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
	// Fixture labels copied from the session response headers. Not credentials.
	// https://labs.ig.com/reference/session
	igFixtureCST = "fixture-cst"
	igFixtureXST = "fixture-xst"

	// dealReference from POST /positions/otc. Not a credential.
	// https://labs.ig.com/reference/positions-otc
	igDealRef = "DREF1"

	koISIN = "US1912161007"

	// igAccepted is the create-position response. dealReference is the broker id.
	// No fill size is returned, so the order is ACCEPTED.
	// https://labs.ig.com/reference/positions-otc
	igAccepted = `{"dealReference":"DREF1"}`

	// igAccountUSD is GET /accounts v1. balance is cash. profitLoss is added for equity.
	// available is not cash. currency is the account currency.
	// https://labs.ig.com/reference/accounts
	igAccountUSD = `{
		"accounts": [{
			"accountId": "Z12345",
			"accountType": "CFD",
			"preferred": true,
			"status": "ENABLED",
			"currency": "USD",
			"balance": {"balance": 1000.50, "deposit": 0, "profitLoss": 1500.25, "available": 1}
		}]
	}`

	// igAccountEUR is the same account shape with a non-USD currency.
	// https://labs.ig.com/reference/accounts
	igAccountEUR = `{
		"accounts": [{
			"accountId": "Z12345",
			"accountType": "CFD",
			"preferred": true,
			"status": "ENABLED",
			"currency": "EUR",
			"balance": {"balance": 1000.50, "deposit": 0, "profitLoss": 1500.25, "available": 1}
		}]
	}`
)

type igBroker struct {
	mu              sync.Mutex
	calls           []recorded
	sessionStatus   int
	sessionBody     string
	accountStatus   int
	accountBody     string
	positionsStatus int
	positionsBody   string
	ordersStatus    int
	ordersBody      string
	placeStatus     int
	placeBody       string
}

func (b *igBroker) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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

	if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/session") && status == http.StatusOK {
		w.Header().Set("CST", igFixtureCST)
		w.Header().Set("X-SECURITY-TOKEN", igFixtureXST)
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

func (b *igBroker) response(r *http.Request) (int, []byte) {
	switch {
	case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/session"):
		status := b.sessionStatus
		if status == 0 {
			status = http.StatusOK
		}
		body := b.sessionBody
		if body == "" && status == http.StatusOK {
			body = `{}`
		}
		return status, []byte(body)
	case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/positions/otc"):
		status := b.placeStatus
		if status == 0 {
			status = http.StatusOK
		}
		body := b.placeBody
		if body == "" && status == http.StatusOK {
			body = igAccepted
		}
		return status, []byte(body)
	case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/accounts"):
		return httpBody(b.accountStatus, b.accountBody)
	case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/positions"):
		return httpBody(b.positionsStatus, b.positionsBody)
	case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/working-orders"):
		return httpBody(b.ordersStatus, b.ordersBody)
	default:
		return http.StatusNotFound, []byte(`{"errorCode":"invalid.url"}`)
	}
}

func (b *igBroker) callsLocked(match func(recorded) bool) []recorded {
	b.mu.Lock()
	defer b.mu.Unlock()
	var out []recorded
	for _, call := range b.calls {
		if match(call) {
			out = append(out, call)
		}
	}
	return out
}

func (b *igBroker) sessionPosts() []recorded {
	return b.callsLocked(func(call recorded) bool {
		return call.method == http.MethodPost && strings.HasSuffix(call.path, "/session")
	})
}

func (b *igBroker) orderPosts() []recorded {
	return b.callsLocked(func(call recorded) bool {
		return call.method == http.MethodPost && strings.HasSuffix(call.path, "/positions/otc")
	})
}

func newIG(t *testing.T, b *igBroker) (*trading.IGClient, *trading.Book) {
	t.Helper()
	book, err := trading.Load("")
	require.NoError(t, err)
	return newIGBook(t, b, book, trading.IGConfig{}), book
}

func newIGBook(t *testing.T, b *igBroker, book *trading.Book, cfg trading.IGConfig) *trading.IGClient {
	t.Helper()
	if b == nil {
		b = &igBroker{}
	}
	srv := httptest.NewServer(b)
	t.Cleanup(srv.Close)
	cfg.BaseURL = srv.URL
	client, err := trading.NewIG(cfg, book, srv.Client())
	require.NoError(t, err)
	return client
}

func aaplIG(t *testing.T, book *trading.Book) trading.Listing {
	t.Helper()
	row, ok := book.Get(aaplISIN)
	require.True(t, ok)
	require.NotEmpty(t, row.IGEpic)
	require.Equal(t, int64(1), row.IGShareSize)
	return row
}

func igMarketBody(row trading.Listing, direction string, size int64) string {
	return fmt.Sprintf(
		`{"currencyCode":%q,"direction":%q,"epic":%q,"expiry":"-","forceOpen":false,"guaranteedStop":false,"orderType":"MARKET","size":%d}`,
		row.Currency, direction, row.IGEpic, size,
	)
}

func igPositionBody(epic string, direction string, size float64, level float64) string {
	return fmt.Sprintf(`{"positions":[{"market":{"epic":%q,"expiry":"-"},"position":{"dealId":"POS1","direction":%q,"level":%g,"size":%g}}]}`,
		epic, direction, level, size)
}

func TestIGMarketBuy(t *testing.T) {
	b := &igBroker{}
	client, book := newIG(t, b)
	row := aaplIG(t, book)

	got, err := client.PlaceOrder(context.Background(), marketOrder(10))
	require.NoError(t, err)
	require.Equal(t, pb.OrderStatus_ORDER_STATUS_ACCEPTED, got.GetStatus())
	require.Equal(t, igDealRef, got.GetBrokerOrderId())
	require.Equal(t, row.ISIN, got.GetIsin())
	require.Equal(t, row.Symbol, got.GetSymbol())
	require.Equal(t, int32(10), got.GetAmount())
	require.Equal(t, int32(0), got.GetFilledAmount())
	require.Equal(t, pb.PositionIntent_POSITION_INTENT_UNSPECIFIED, got.GetIntent())

	sessions := b.sessionPosts()
	require.Len(t, sessions, 1)
	require.Equal(t, "/session", sessions[0].path)
	require.Equal(t, "2", sessions[0].header.Get("Version"))
	require.Equal(t, "application/json; charset=UTF-8", sessions[0].header.Get("Accept"))
	require.Equal(t, "application/json", sessions[0].header.Get("Content-Type"))
	require.Empty(t, sessions[0].header.Get("X-IG-API-KEY"))
	require.Empty(t, sessions[0].header.Get("CST"))
	require.Empty(t, sessions[0].header.Get("X-SECURITY-TOKEN"))
	// identifier, password, encryptedPassword false. Empty secrets stay empty.
	// https://labs.ig.com/reference/session
	require.JSONEq(t, `{"identifier":"","password":"","encryptedPassword":false}`, string(sessions[0].body))

	posts := b.orderPosts()
	require.Len(t, posts, 1)
	require.Equal(t, "/positions/otc", posts[0].path)
	require.Equal(t, "2", posts[0].header.Get("Version"))
	require.Equal(t, igFixtureCST, posts[0].header.Get("CST"))
	require.Equal(t, igFixtureXST, posts[0].header.Get("X-SECURITY-TOKEN"))
	require.Empty(t, posts[0].header.Get("Authorization"))
	// epic, direction BUY, size 10. level is omitted for MARKET.
	// https://labs.ig.com/reference/positions-otc
	require.Equal(t, igMarketBody(row, "BUY", 10), string(posts[0].body))

	var fields map[string]any
	require.NoError(t, json.Unmarshal(posts[0].body, &fields))
	require.Equal(t, row.IGEpic, fields["epic"])
	require.Equal(t, "BUY", fields["direction"])
	require.InDelta(t, 10, fields["size"], 0)
	require.Equal(t, "MARKET", fields["orderType"])
	_, hasLevel := fields["level"]
	require.False(t, hasLevel)
	require.NotContains(t, string(posts[0].body), "NOT_SENT")
	require.NotContains(t, string(posts[0].body), "intent")
	require.NotContains(t, string(posts[0].body), "stopLevel")
}

func TestIGMarketSell(t *testing.T) {
	b := &igBroker{}
	client, book := newIG(t, b)
	row := aaplIG(t, book)

	got, err := client.PlaceOrder(context.Background(), marketOrder(-10))
	require.NoError(t, err)
	require.Equal(t, int32(-10), got.GetAmount())
	require.Equal(t, pb.OrderStatus_ORDER_STATUS_ACCEPTED, got.GetStatus())
	require.NotEmpty(t, got.GetBrokerOrderId())

	posts := b.orderPosts()
	require.Len(t, posts, 1)
	// SELL is the negative amount. size stays positive.
	// https://labs.ig.com/reference/positions-otc
	require.Equal(t, igMarketBody(row, "SELL", 10), string(posts[0].body))
}

func TestIGLimitPrice(t *testing.T) {
	b := &igBroker{}
	client, book := newIG(t, b)
	row := aaplIG(t, book)

	order := marketOrder(10)
	order.Type = pb.OrderType_ORDER_TYPE_LIMIT
	order.LimitPrice = 150
	got, err := client.PlaceOrder(context.Background(), order)
	require.NoError(t, err)
	require.Equal(t, pb.OrderType_ORDER_TYPE_LIMIT, got.GetType())
	require.InDelta(t, 150, got.GetLimitPrice(), 1e-9)

	posts := b.orderPosts()
	require.Len(t, posts, 1)
	// LIMIT sets level. MARKET must not.
	// https://labs.ig.com/reference/positions-otc
	want := fmt.Sprintf(
		`{"currencyCode":%q,"direction":"BUY","epic":%q,"expiry":"-","forceOpen":false,"guaranteedStop":false,"level":150,"orderType":"LIMIT","size":10}`,
		row.Currency, row.IGEpic,
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

func TestIGUnknownISIN(t *testing.T) {
	b := &igBroker{}
	client, _ := newIG(t, b)
	order := marketOrder(10)
	order.Isin = "US0000000000"
	placed, err := client.PlaceOrder(context.Background(), order)
	require.ErrorIs(t, err, supplier.ErrNotTradable)
	require.Nil(t, placed)
	require.Empty(t, b.orderPosts())
}

func TestIGZeroAmount(t *testing.T) {
	b := &igBroker{}
	client, _ := newIG(t, b)
	placed, err := client.PlaceOrder(context.Background(), marketOrder(0))
	require.ErrorIs(t, err, supplier.ErrRejected)
	require.Nil(t, placed)
	require.Empty(t, b.orderPosts())
}

func TestIGBrokerHTTP400(t *testing.T) {
	// errorCode is the 4xx body.
	// https://labs.ig.com/rest-trading-api-guide
	b := &igBroker{
		placeStatus: http.StatusBadRequest,
		placeBody:   `{"errorCode":"invalid.input"}`,
	}
	client, _ := newIG(t, b)
	placed, err := client.PlaceOrder(context.Background(), marketOrder(10))
	require.ErrorIs(t, err, supplier.ErrRejected)
	require.Nil(t, placed)
	require.Len(t, b.orderPosts(), 1)
	require.Len(t, b.sessionPosts(), 1)
}

func TestIGBrokerHTTP500(t *testing.T) {
	b := &igBroker{placeStatus: http.StatusInternalServerError}
	client, _ := newIG(t, b)
	placed, err := client.PlaceOrder(context.Background(), marketOrder(10))
	require.ErrorIs(t, err, supplier.ErrUnavailable)
	require.Nil(t, placed)
	require.Len(t, b.orderPosts(), 1)
	require.Len(t, b.sessionPosts(), 1)
}

func TestIGCanceledContext(t *testing.T) {
	b := &igBroker{}
	client, _ := newIG(t, b)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	placed, err := client.PlaceOrder(ctx, marketOrder(10))
	require.ErrorIs(t, err, context.Canceled)
	require.Nil(t, placed)
	require.Empty(t, b.orderPosts())
	require.Len(t, b.sessionPosts(), 1)
}

func TestIGGetPortfolio(t *testing.T) {
	b := &igBroker{
		accountBody:   igAccountUSD,
		positionsBody: igPositionBody("UA.D.AAPL.DAILY.IP", "BUY", 5, 500),
	}
	client, book := newIG(t, b)
	row := aaplIG(t, book)

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
	require.Len(t, b.sessionPosts(), 1)

	var sawAccounts, sawPositions bool
	for _, call := range b.callsLocked(func(recorded) bool { return true }) {
		if call.method == http.MethodGet && call.path == "/accounts" {
			sawAccounts = true
			require.Equal(t, "1", call.header.Get("Version"))
			require.Equal(t, igFixtureCST, call.header.Get("CST"))
		}
		if call.method == http.MethodGet && call.path == "/positions" {
			sawPositions = true
			require.Equal(t, "2", call.header.Get("Version"))
		}
	}
	require.True(t, sawAccounts)
	require.True(t, sawPositions)
}

func TestIGUnmappedPosition(t *testing.T) {
	b := &igBroker{
		accountBody: igAccountUSD,
		// epic is not an ig_epic in the sample listing.
		// https://labs.ig.com/reference/positions
		positionsBody: igPositionBody("ZZ.D.NONE.DAILY.IP", "BUY", 1, 1),
	}
	client, _ := newIG(t, b)
	got, err := client.GetPortfolio(context.Background())
	require.ErrorIs(t, err, supplier.ErrUnmappedPosition)
	require.Nil(t, got)
}

func TestIGCurrencyMismatch(t *testing.T) {
	b := &igBroker{accountBody: igAccountEUR}
	client, _ := newIG(t, b)
	got, err := client.GetPortfolio(context.Background())
	require.ErrorIs(t, err, supplier.ErrCurrency)
	require.Nil(t, got)
}

func TestIGGetOrders(t *testing.T) {
	b := &igBroker{}
	client, book := newIG(t, b)
	row := aaplIG(t, book)
	// workingOrders is the open-order list. dealId is the broker id.
	// orderType LIMIT and orderLevel are the working limit. GOOD_TILL_DATE is the day order.
	// https://labs.ig.com/reference/working-orders
	b.ordersBody = fmt.Sprintf(`{"workingOrders":[{
		"workingOrderData": {
			"dealId": "WO1",
			"direction": "BUY",
			"epic": %q,
			"orderSize": 2,
			"orderLevel": 150,
			"orderType": "LIMIT",
			"timeInForce": "GOOD_TILL_DATE"
		},
		"marketData": {"epic": %q}
	}]}`, row.IGEpic, row.IGEpic)

	got, err := client.GetOrders(context.Background())
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, "WO1", got[0].GetBrokerOrderId())
	require.Equal(t, row.ISIN, got[0].GetIsin())
	require.Equal(t, row.Symbol, got[0].GetSymbol())
	require.Equal(t, int32(2), got[0].GetAmount())
	require.Equal(t, int32(0), got[0].GetFilledAmount())
	require.Equal(t, pb.OrderStatus_ORDER_STATUS_ACCEPTED, got[0].GetStatus())
	require.Equal(t, pb.OrderType_ORDER_TYPE_LIMIT, got[0].GetType())
	require.Equal(t, pb.TimeInForce_TIME_IN_FORCE_DAY, got[0].GetTimeInForce())
	require.InDelta(t, 150, got[0].GetLimitPrice(), 1e-9)

	var sawOpen bool
	for _, call := range b.callsLocked(func(recorded) bool { return true }) {
		if call.method == http.MethodGet && call.path == "/working-orders" {
			sawOpen = true
			require.Equal(t, "2", call.header.Get("Version"))
		}
	}
	require.True(t, sawOpen)

	emptyBroker := &igBroker{ordersBody: `{"workingOrders":[]}`}
	emptyClient, _ := newIG(t, emptyBroker)
	none, err := emptyClient.GetOrders(context.Background())
	require.NoError(t, err)
	require.Empty(t, none)
}

func TestIGUnsized(t *testing.T) {
	b := &igBroker{}
	client, book := newIG(t, b)
	row, ok := book.Get(koISIN)
	require.True(t, ok)
	require.Equal(t, int64(10), row.IGShareSize)
	require.NotEmpty(t, row.IGEpic)

	order := marketOrder(15)
	order.Isin = row.ISIN
	placed, err := client.PlaceOrder(context.Background(), order)
	require.ErrorIs(t, err, supplier.ErrUnsized)
	require.Nil(t, placed)

	sell := marketOrder(-5)
	sell.Isin = row.ISIN
	placed, err = client.PlaceOrder(context.Background(), sell)
	require.ErrorIs(t, err, supplier.ErrUnsized)
	require.Nil(t, placed)
	require.Empty(t, b.orderPosts())
	require.Len(t, b.sessionPosts(), 1)
}

func TestIGShareSize(t *testing.T) {
	b := &igBroker{}
	client, book := newIG(t, b)
	row, ok := book.Get(koISIN)
	require.True(t, ok)

	order := marketOrder(20)
	order.Isin = row.ISIN
	got, err := client.PlaceOrder(context.Background(), order)
	require.NoError(t, err)
	require.Equal(t, int32(20), got.GetAmount())
	posts := b.orderPosts()
	require.Len(t, posts, 1)
	// 20 shares / ig_share_size 10 is deal size 2. The epic is the listing epic.
	// https://labs.ig.com/reference/positions-otc
	require.Equal(t, igMarketBody(row, "BUY", 2), string(posts[0].body))

	b.accountBody = igAccountUSD
	b.positionsBody = igPositionBody(row.IGEpic, "SELL", 1, 40)
	portfolio, err := client.GetPortfolio(context.Background())
	require.NoError(t, err)
	require.Len(t, portfolio.GetPositions(), 1)
	require.Equal(t, row.ISIN, portfolio.GetPositions()[0].GetIsin())
	require.Equal(t, int32(-10), portfolio.GetPositions()[0].GetAmount())
}

func TestIGEmptyISIN(t *testing.T) {
	b := &igBroker{}
	client, _ := newIG(t, b)
	order := marketOrder(10)
	order.Isin = "  "
	placed, err := client.PlaceOrder(context.Background(), order)
	require.ErrorIs(t, err, supplier.ErrNotTradable)
	require.Nil(t, placed)
	require.Empty(t, b.orderPosts())
}

func TestIGEmptyEpic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "listings.json")
	raw := []byte(`[{
		"isin": "US0378331005",
		"symbol": "AAPL",
		"currency": "USD",
		"alpaca_symbol": "AAPL",
		"saxo_uic": 0,
		"saxo_asset_type": "",
		"ibkr_conid": 0,
		"ig_epic": "",
		"ig_share_size": 1
	}]`)
	require.NoError(t, os.WriteFile(path, raw, 0o644))
	book, err := trading.Load(path)
	require.NoError(t, err)

	b := &igBroker{}
	client := newIGBook(t, b, book, trading.IGConfig{})
	placed, err := client.PlaceOrder(context.Background(), marketOrder(10))
	require.ErrorIs(t, err, supplier.ErrNotTradable)
	require.Nil(t, placed)
	require.Empty(t, b.orderPosts())
}

func TestIGListingEpic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "listings.json")
	raw := []byte(`[{
		"isin": "US0378331005",
		"symbol": "AAPL",
		"currency": "USD",
		"alpaca_symbol": "AAPL",
		"saxo_uic": 0,
		"saxo_asset_type": "",
		"ibkr_conid": 0,
		"ig_epic": "UA.D.TEST.DAILY.IP",
		"ig_share_size": 1
	}]`)
	require.NoError(t, os.WriteFile(path, raw, 0o644))
	book, err := trading.Load(path)
	require.NoError(t, err)

	b := &igBroker{}
	client := newIGBook(t, b, book, trading.IGConfig{})
	_, err = client.PlaceOrder(context.Background(), marketOrder(10))
	require.NoError(t, err)
	posts := b.orderPosts()
	require.Len(t, posts, 1)
	require.Contains(t, string(posts[0].body), `"epic":"UA.D.TEST.DAILY.IP"`)
	require.NotContains(t, string(posts[0].body), "UA.D.AAPL.DAILY.IP")
}

func TestIGBrokerHTTP401(t *testing.T) {
	b := &igBroker{placeStatus: http.StatusUnauthorized}
	client, _ := newIG(t, b)
	placed, err := client.PlaceOrder(context.Background(), marketOrder(10))
	require.ErrorIs(t, err, supplier.ErrUnavailable)
	require.Nil(t, placed)
	require.Len(t, b.orderPosts(), 1)
	require.Len(t, b.sessionPosts(), 1)
}

func TestIGSessionDoesNotRetry(t *testing.T) {
	b := &igBroker{sessionStatus: http.StatusUnauthorized}
	srv := httptest.NewServer(b)
	t.Cleanup(srv.Close)
	book, err := trading.Load("")
	require.NoError(t, err)
	_, err = trading.NewIG(trading.IGConfig{BaseURL: srv.URL}, book, srv.Client())
	require.ErrorIs(t, err, supplier.ErrUnavailable)
	require.Len(t, b.sessionPosts(), 1)
	require.Empty(t, b.orderPosts())
}

func TestIGLocalRejects(t *testing.T) {
	b := &igBroker{}
	client, _ := newIG(t, b)

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

	placed, err = client.PlaceOrder(context.Background(), nil)
	require.ErrorIs(t, err, supplier.ErrRejected)
	require.Nil(t, placed)

	overflow := marketOrder(math.MinInt32)
	placed, err = client.PlaceOrder(context.Background(), overflow)
	require.ErrorIs(t, err, supplier.ErrRejected)
	require.Nil(t, placed)
	require.Empty(t, b.orderPosts())
}

func TestIGShortPositionIsNegative(t *testing.T) {
	b := &igBroker{
		accountBody: igAccountUSD,
		// direction SELL and size are the short. size is a positive deal size.
		// https://labs.ig.com/reference/positions
		positionsBody: igPositionBody("UA.D.MSFT.DAILY.IP", "SELL", 3, 90),
	}
	client, book := newIG(t, b)
	row, ok := book.Get(msftISIN)
	require.True(t, ok)

	got, err := client.GetPortfolio(context.Background())
	require.NoError(t, err)
	require.Len(t, got.GetPositions(), 1)
	require.Equal(t, row.ISIN, got.GetPositions()[0].GetIsin())
	require.Equal(t, row.Symbol, got.GetPositions()[0].GetSymbol())
	require.Equal(t, int32(-3), got.GetPositions()[0].GetAmount())
}

func TestIGDemoHostDefault(t *testing.T) {
	book, err := trading.Load("")
	require.NoError(t, err)
	var seen *http.Request
	client := &http.Client{Transport: igRoundTrip(func(r *http.Request) (*http.Response, error) {
		seen = r
		return igSessionResponse(), nil
	})}
	_, err = trading.NewIG(trading.IGConfig{}, book, client)
	require.NoError(t, err)
	require.NotNil(t, seen)
	require.Equal(t, "https", seen.URL.Scheme)
	require.Equal(t, "demo-api.ig.com", seen.URL.Host)
	require.Equal(t, "/gateway/deal/session", seen.URL.Path)
	require.Equal(t, "2", seen.Header.Get("Version"))
	require.Empty(t, seen.Header.Get("CST"))
}

func TestIGLiveHostRefusedUnlessAllowed(t *testing.T) {
	book, err := trading.Load("")
	require.NoError(t, err)
	blocked := &http.Client{Transport: igRoundTrip(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("unexpected http")
	})}
	_, err = trading.NewIG(trading.IGConfig{
		BaseURL: "https://api.ig.com/gateway/deal",
	}, book, blocked)
	require.Error(t, err)
	require.ErrorContains(t, err, "refused")
	require.NotErrorIs(t, err, supplier.ErrUnavailable)

	var host string
	allowed := &http.Client{Transport: igRoundTrip(func(r *http.Request) (*http.Response, error) {
		host = r.URL.Host
		return igSessionResponse(), nil
	})}
	_, err = trading.NewIG(trading.IGConfig{
		BaseURL:      "https://api.ig.com/gateway/deal",
		AllowLiveURL: true,
	}, book, allowed)
	require.NoError(t, err)
	require.Equal(t, "api.ig.com", host)

	demo := &http.Client{Transport: igRoundTrip(func(r *http.Request) (*http.Response, error) {
		require.Equal(t, "demo-api.ig.com", r.URL.Host)
		return igSessionResponse(), nil
	})}
	_, err = trading.NewIG(trading.IGConfig{
		BaseURL: "https://demo-api.ig.com/gateway/deal",
	}, book, demo)
	require.NoError(t, err)

	_, err = trading.NewIG(trading.IGConfig{
		BaseURL: "https://alice@demo-api.ig.com/gateway/deal",
	}, book, blocked)
	require.Error(t, err)
	require.ErrorContains(t, err, "userinfo")
	require.NotErrorIs(t, err, supplier.ErrUnavailable)
}

type igRoundTrip func(*http.Request) (*http.Response, error)

func (f igRoundTrip) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func igSessionResponse() *http.Response {
	header := make(http.Header)
	header.Set("Content-Type", "application/json")
	header.Set("CST", igFixtureCST)
	header.Set("X-SECURITY-TOKEN", igFixtureXST)
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     header,
		Body:       io.NopCloser(strings.NewReader(`{}`)),
	}
}
