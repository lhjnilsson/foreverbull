package trading_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
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
	// ibkrAccountID is the account id in the Client Portal gateway examples.
	// It is a documentation placeholder, not a credential.
	// https://www.interactivebrokers.com/docs/web-api/api/web-api/placing-orders
	ibkrAccountID = "DU123456"

	// ibkrAccepted is a placed order. order_id is a string. order_status Submitted
	// is working and has no fill size.
	// https://www.interactivebrokers.com/docs/web-api/v1/endpoints/orders/place-order
	ibkrAccepted = `[{"order_id":"1234567890","order_status":"Submitted","encrypt_message":"1"}]`

	// ibkrPrompt is the alternate place response. id is confirmed once.
	// https://www.interactivebrokers.com/docs/web-api/v1/endpoints/orders/place-order
	// https://www.interactivebrokers.com/docs/web-api/trading/orders/order-reply-messages
	ibkrPrompt = `[{
		"id": "07a13a5a-4a48-44a5-bb25-5ab37b79186c",
		"message": ["Are you sure you want to submit this order?"],
		"isSuppressed": false,
		"messageIds": ["o163"]
	}]`

	// ibkrSecondPrompt is another reply message. It must not be posted again.
	// https://www.interactivebrokers.com/docs/web-api/v1/endpoints/orders/place-order-reply-confirmation
	ibkrSecondPrompt = `[{
		"id": "a12b34c5-d678-9e012f-3456-7a890b12cd3e",
		"message": ["A second confirmation is required."],
		"isSuppressed": false,
		"messageIds": ["o354"]
	}]`

	// ibkrAccountUSD is one gateway account. currency is the base currency.
	// https://www.interactivebrokers.com/docs/web-api/v1/endpoints/portfolio/portfolio-accounts
	ibkrAccountUSD = `[{"id":"DU123456","accountId":"DU123456","currency":"USD","type":"DEMO"}]`

	// ibkrAccountEUR is the same account shape with a non-USD currency.
	// https://www.interactivebrokers.com/docs/web-api/v1/endpoints/portfolio/portfolio-accounts
	ibkrAccountEUR = `[{"id":"DU123456","accountId":"DU123456","currency":"EUR","type":"DEMO"}]`

	// ibkrLedgerUSD is GET /portfolio/{accountId}/ledger. cashbalance is cash.
	// netliquidationvalue is the portfolio value. BASE is not the account currency.
	// https://www.interactivebrokers.com/docs/web-api/v1/endpoints/portfolio/portfolio-ledger
	ibkrLedgerUSD = `{
		"USD": {"cashbalance": 1000.50, "netliquidationvalue": 2500.75, "currency": "USD"},
		"BASE": {"cashbalance": 1, "netliquidationvalue": 2, "currency": "BASE"}
	}`
)

const (
	ibkrReplyID  = "07a13a5a-4a48-44a5-bb25-5ab37b79186c"
	ibkrSecondID = "a12b34c5-d678-9e012f-3456-7a890b12cd3e"
)

type ibkrBroker struct {
	mu              sync.Mutex
	calls           []recorded
	accountStatus   int
	accountBody     string
	ledgerStatus    int
	ledgerBody      string
	positionsStatus int
	positionsBody   string
	ordersStatus    int
	ordersBody      string
	placeStatus     int
	placeBodies     []string
	replyStatus     int
	replyBodies     []string
}

func (b *ibkrBroker) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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

func (b *ibkrBroker) response(r *http.Request) (int, []byte) {
	switch {
	case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/iserver/account/") && strings.HasSuffix(r.URL.Path, "/orders"):
		status := b.placeStatus
		if status == 0 {
			status = http.StatusOK
		}
		body := takeBody(&b.placeBodies)
		if body == "" && status == http.StatusOK {
			body = ibkrAccepted
		}
		return status, []byte(body)
	case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/iserver/reply/"):
		status := b.replyStatus
		if status == 0 {
			status = http.StatusOK
		}
		body := takeBody(&b.replyBodies)
		if body == "" && status == http.StatusOK {
			body = ibkrAccepted
		}
		return status, []byte(body)
	case r.Method == http.MethodGet && r.URL.Path == "/v1/api/portfolio/accounts":
		return httpBody(b.accountStatus, b.accountBody)
	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/v1/api/portfolio/") && strings.HasSuffix(r.URL.Path, "/ledger"):
		return httpBody(b.ledgerStatus, b.ledgerBody)
	case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/portfolio2/") && strings.HasSuffix(r.URL.Path, "/positions"):
		return httpBody(b.positionsStatus, b.positionsBody)
	case r.Method == http.MethodGet && r.URL.Path == "/v1/api/iserver/account/orders":
		return httpBody(b.ordersStatus, b.ordersBody)
	default:
		return http.StatusNotFound, []byte(`{"error":"not found"}`)
	}
}

func takeBody(queue *[]string) string {
	if len(*queue) == 0 {
		return ""
	}
	body := (*queue)[0]
	*queue = (*queue)[1:]
	return body
}

func (b *ibkrBroker) orderPosts() []recorded {
	b.mu.Lock()
	defer b.mu.Unlock()
	var out []recorded
	for _, call := range b.calls {
		if call.method == http.MethodPost && strings.Contains(call.path, "/iserver/account/") && strings.HasSuffix(call.path, "/orders") {
			out = append(out, call)
		}
	}
	return out
}

func newIBKR(t *testing.T, b *ibkrBroker) (*trading.IBKRClient, *trading.Book) {
	t.Helper()
	book, err := trading.Load("")
	require.NoError(t, err)
	return newIBKRBook(t, b, book, trading.IBKRConfig{}), book
}

func newIBKRBook(t *testing.T, b *ibkrBroker, book *trading.Book, cfg trading.IBKRConfig) *trading.IBKRClient {
	t.Helper()
	if b == nil {
		b = &ibkrBroker{}
	}
	srv := httptest.NewServer(b)
	t.Cleanup(srv.Close)
	if cfg.AccountMode == "" {
		cfg.AccountMode = "paper"
	}
	if cfg.AccountID == "" {
		cfg.AccountID = ibkrAccountID
	}
	cfg.BaseURL = srv.URL + "/v1/api"
	client, err := trading.NewIBKR(cfg, book, srv.Client())
	require.NoError(t, err)
	return client
}

func aaplIBKR(t *testing.T, book *trading.Book) trading.Listing {
	t.Helper()
	row, ok := book.Get(aaplISIN)
	require.True(t, ok)
	require.NotZero(t, row.IBKRConID)
	return row
}

func ibkrMarketBody(row trading.Listing, side string, qty int32) string {
	return fmt.Sprintf(
		`{"orders":[{"conid":%d,"orderType":"MKT","quantity":%d,"side":%q,"tif":"DAY"}]}`,
		row.IBKRConID, qty, side,
	)
}

func ibkrPositionBody(conid int64, qty int32, avgCost float64) string {
	return fmt.Sprintf(`[{"conid":%d,"position":%d,"avgCost":%g,"currency":"USD"}]`, conid, qty, avgCost)
}

func TestIBKRMarketBuy(t *testing.T) {
	b := &ibkrBroker{}
	client, book := newIBKR(t, b)
	row := aaplIBKR(t, book)

	got, err := client.PlaceOrder(context.Background(), marketOrder(10))
	require.NoError(t, err)
	require.Equal(t, pb.OrderStatus_ORDER_STATUS_ACCEPTED, got.GetStatus())
	require.Equal(t, "1234567890", got.GetBrokerOrderId())
	require.Equal(t, row.ISIN, got.GetIsin())
	require.Equal(t, row.Symbol, got.GetSymbol())
	require.Equal(t, int32(10), got.GetAmount())
	require.Equal(t, int32(0), got.GetFilledAmount())
	require.Equal(t, pb.PositionIntent_POSITION_INTENT_UNSPECIFIED, got.GetIntent())

	posts := b.orderPosts()
	require.Len(t, posts, 1)
	require.Empty(t, replyCalls(b))
	require.Equal(t, "/v1/api/iserver/account/"+ibkrAccountID+"/orders", posts[0].path)
	// orders wrapper, conid, side BUY, quantity, orderType MKT, tif DAY. price is omitted.
	// https://www.interactivebrokers.com/docs/web-api/api/web-api/placing-orders
	// https://www.interactivebrokers.com/docs/web-api/v1/endpoints/orders/place-order
	require.Equal(t, ibkrMarketBody(row, "BUY", 10), string(posts[0].body))

	var fields map[string]any
	require.NoError(t, json.Unmarshal(posts[0].body, &fields))
	orders, ok := fields["orders"].([]any)
	require.True(t, ok)
	require.Len(t, orders, 1)
	ticket := orders[0].(map[string]any)
	require.InDelta(t, float64(row.IBKRConID), ticket["conid"], 0)
	require.Equal(t, "BUY", ticket["side"])
	require.InDelta(t, 10, ticket["quantity"], 0)
	require.Equal(t, "MKT", ticket["orderType"])
	require.Equal(t, "DAY", ticket["tif"])
	_, hasPrice := ticket["price"]
	require.False(t, hasPrice)
	require.NotContains(t, string(posts[0].body), "NOT_SENT")
	require.NotContains(t, string(posts[0].body), "cashQty")
	require.Empty(t, posts[0].header.Get("Authorization"))
	require.Equal(t, "application/json", posts[0].header.Get("Content-Type"))
}

func TestIBKRMarketSell(t *testing.T) {
	b := &ibkrBroker{}
	client, book := newIBKR(t, b)
	row := aaplIBKR(t, book)

	got, err := client.PlaceOrder(context.Background(), marketOrder(-10))
	require.NoError(t, err)
	require.Equal(t, int32(-10), got.GetAmount())
	require.Equal(t, pb.OrderStatus_ORDER_STATUS_ACCEPTED, got.GetStatus())
	require.NotEmpty(t, got.GetBrokerOrderId())

	posts := b.orderPosts()
	require.Len(t, posts, 1)
	// SELL keeps quantity positive.
	// https://www.interactivebrokers.com/docs/web-api/trading/orders/new-order-example
	require.Equal(t, ibkrMarketBody(row, "SELL", 10), string(posts[0].body))
	require.NotContains(t, string(posts[0].body), "NOT_SENT")
}

func TestIBKRLimitPrice(t *testing.T) {
	b := &ibkrBroker{}
	client, book := newIBKR(t, b)
	row := aaplIBKR(t, book)

	order := marketOrder(10)
	order.Type = pb.OrderType_ORDER_TYPE_LIMIT
	order.LimitPrice = 150
	got, err := client.PlaceOrder(context.Background(), order)
	require.NoError(t, err)
	require.Equal(t, pb.OrderType_ORDER_TYPE_LIMIT, got.GetType())
	require.InDelta(t, 150, got.GetLimitPrice(), 1e-9)

	posts := b.orderPosts()
	require.Len(t, posts, 1)
	// price is the limit and is omitted for MKT.
	// https://www.interactivebrokers.com/docs/web-api/api/web-api/placing-orders
	want := fmt.Sprintf(
		`{"orders":[{"conid":%d,"orderType":"LMT","price":150,"quantity":10,"side":"BUY","tif":"DAY"}]}`,
		row.IBKRConID,
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

func TestIBKRUnknownISIN(t *testing.T) {
	b := &ibkrBroker{}
	client, _ := newIBKR(t, b)
	order := marketOrder(10)
	order.Isin = "US0000000000"
	placed, err := client.PlaceOrder(context.Background(), order)
	require.ErrorIs(t, err, supplier.ErrNotTradable)
	require.Nil(t, placed)
	require.Empty(t, b.orderPosts())
}

func TestIBKRZeroAmount(t *testing.T) {
	b := &ibkrBroker{}
	client, _ := newIBKR(t, b)
	placed, err := client.PlaceOrder(context.Background(), marketOrder(0))
	require.ErrorIs(t, err, supplier.ErrRejected)
	require.Nil(t, placed)
	require.Empty(t, b.orderPosts())
}

func TestIBKRBrokerHTTP400(t *testing.T) {
	b := &ibkrBroker{placeStatus: http.StatusBadRequest, placeBodies: []string{`{"error":"rejected"}`}}
	client, _ := newIBKR(t, b)
	placed, err := client.PlaceOrder(context.Background(), marketOrder(10))
	require.ErrorIs(t, err, supplier.ErrRejected)
	require.Nil(t, placed)
	require.Len(t, b.orderPosts(), 1)
	require.Empty(t, replyCalls(b))
}

func TestIBKRBrokerHTTP500(t *testing.T) {
	b := &ibkrBroker{placeStatus: http.StatusInternalServerError}
	client, _ := newIBKR(t, b)
	placed, err := client.PlaceOrder(context.Background(), marketOrder(10))
	require.ErrorIs(t, err, supplier.ErrUnavailable)
	require.Nil(t, placed)
	require.Len(t, b.orderPosts(), 1)
}

func TestIBKRCanceledContext(t *testing.T) {
	b := &ibkrBroker{}
	client, _ := newIBKR(t, b)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	placed, err := client.PlaceOrder(ctx, marketOrder(10))
	require.ErrorIs(t, err, context.Canceled)
	require.Nil(t, placed)
	require.Empty(t, b.orderPosts())
	require.Empty(t, b.calls)
}

func TestIBKRGetPortfolio(t *testing.T) {
	b := &ibkrBroker{accountBody: ibkrAccountUSD, ledgerBody: ibkrLedgerUSD}
	client, book := newIBKR(t, b)
	row := aaplIBKR(t, book)
	// position is the signed size. conid is the contract. avgCost is copied.
	// https://www.interactivebrokers.com/docs/web-api/v1/endpoints/portfolio/positions-new
	// https://www.interactivebrokers.com/docs/web-api/api-reference/trading/trading-portfolio/get-uncached-positions
	b.positionsBody = ibkrPositionBody(row.IBKRConID, 5, 500)

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

	require.Equal(t, []string{
		"GET /v1/api/portfolio/accounts",
		"GET /v1/api/portfolio/" + ibkrAccountID + "/ledger",
		"GET /v1/api/portfolio2/" + ibkrAccountID + "/positions",
	}, callLines(b.calls))
}

func TestIBKRUnmappedPosition(t *testing.T) {
	b := &ibkrBroker{
		accountBody: ibkrAccountUSD,
		ledgerBody:  ibkrLedgerUSD,
		// conid is not an ibkr_conid in the sample listing.
		// https://www.interactivebrokers.com/docs/web-api/v1/endpoints/portfolio/positions-new
		positionsBody: `[{"conid":999999,"position":1,"avgCost":10}]`,
	}
	client, _ := newIBKR(t, b)
	got, err := client.GetPortfolio(context.Background())
	require.ErrorIs(t, err, supplier.ErrUnmappedPosition)
	require.Nil(t, got)
}

func TestIBKRCurrencyMismatch(t *testing.T) {
	b := &ibkrBroker{accountBody: ibkrAccountEUR, ledgerBody: ibkrLedgerUSD}
	client, _ := newIBKR(t, b)
	got, err := client.GetPortfolio(context.Background())
	require.ErrorIs(t, err, supplier.ErrCurrency)
	require.Nil(t, got)
	require.Equal(t, []string{"GET /v1/api/portfolio/accounts"}, callLines(b.calls))
}

func TestIBKRGetOrders(t *testing.T) {
	b := &ibkrBroker{}
	client, book := newIBKR(t, b)
	row := aaplIBKR(t, book)
	// GET /iserver/account/orders. orderId is the broker id. side and orderType
	// are the working order. filledQuantity 0 is unfilled. price is the limit.
	// https://www.interactivebrokers.com/docs/web-api/v1/endpoints/order-monitoring/live-orders
	// https://www.interactivebrokers.com/docs/web-api/v1/ws/order-position-operations/request-live-order-updates
	b.ordersBody = fmt.Sprintf(`{"orders":[{
		"conid": %d,
		"orderId": 1234568790,
		"remainingQuantity": 2.0,
		"filledQuantity": 0.0,
		"totalSize": 2.0,
		"status": "Submitted",
		"orderType": "Limit",
		"origOrderType": "LIMIT",
		"timeInForce": "DAY",
		"side": "BUY",
		"price": 150
	}], "snapshot": true}`, row.IBKRConID)

	got, err := client.GetOrders(context.Background())
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, "1234568790", got[0].GetBrokerOrderId())
	require.Equal(t, row.ISIN, got[0].GetIsin())
	require.Equal(t, row.Symbol, got[0].GetSymbol())
	require.Equal(t, int32(2), got[0].GetAmount())
	require.Equal(t, pb.OrderStatus_ORDER_STATUS_ACCEPTED, got[0].GetStatus())
	require.Equal(t, pb.OrderType_ORDER_TYPE_LIMIT, got[0].GetType())
	require.Equal(t, pb.TimeInForce_TIME_IN_FORCE_DAY, got[0].GetTimeInForce())
	require.Equal(t, int32(0), got[0].GetFilledAmount())
	require.InDelta(t, 150, got[0].GetLimitPrice(), 1e-9)

	require.NotEmpty(t, b.calls)
	var sawOpen bool
	for _, call := range b.calls {
		if call.method == http.MethodGet && call.path == "/v1/api/iserver/account/orders" {
			sawOpen = true
			values, err := url.ParseQuery(call.query)
			require.NoError(t, err)
			require.Equal(t, ibkrAccountID, values.Get("accountId"))
			require.Equal(t, "pending_submit,pre_submitted,submitted", values.Get("filters"))
			require.Empty(t, values.Get("force"))
		}
	}
	require.True(t, sawOpen)

	emptyBroker := &ibkrBroker{ordersBody: `{"orders":[]}`}
	emptyClient, _ := newIBKR(t, emptyBroker)
	none, err := emptyClient.GetOrders(context.Background())
	require.NoError(t, err)
	require.Empty(t, none)
}

func TestIBKRLimitOrderZeroOrMissingPrice(t *testing.T) {
	b := &ibkrBroker{}
	client, book := newIBKR(t, b)
	row := aaplIBKR(t, book)
	// price is the limit on a live LMT order. Zero, negative, and omitted are not a price.
	// https://www.interactivebrokers.com/docs/web-api/v1/ws/order-position-operations/request-live-order-updates
	order := func(price string) string {
		return fmt.Sprintf(`{"orders":[{
			"conid": %d,
			"orderId": 1234568790,
			"remainingQuantity": 2.0,
			"filledQuantity": 0.0,
			"totalSize": 2.0,
			"status": "Submitted",
			"orderType": "LMT",
			"timeInForce": "DAY",
			"side": "BUY"%s
		}]}`, row.IBKRConID, price)
	}
	bodies := []string{
		order(`, "price": 0`),
		order(`, "price": -1`),
		order(`, "price": null`),
		order(""),
	}
	for _, body := range bodies {
		b.ordersBody = body
		got, err := client.GetOrders(context.Background())
		require.ErrorIs(t, err, supplier.ErrUnavailable)
		require.Nil(t, got)
	}
}

func TestIBKRConfirmOnce(t *testing.T) {
	b := &ibkrBroker{placeBodies: []string{ibkrPrompt}, replyBodies: []string{ibkrAccepted}}
	client, book := newIBKR(t, b)
	row := aaplIBKR(t, book)

	got, err := client.PlaceOrder(context.Background(), marketOrder(10))
	require.NoError(t, err)
	require.Equal(t, pb.OrderStatus_ORDER_STATUS_ACCEPTED, got.GetStatus())
	require.Equal(t, "1234567890", got.GetBrokerOrderId())
	require.Equal(t, row.ISIN, got.GetIsin())

	require.Len(t, b.orderPosts(), 1)
	replies := replyCalls(b)
	require.Len(t, replies, 1)
	require.Equal(t, "/v1/api/iserver/reply/"+ibkrReplyID, replies[0].path)
	// confirmed true is the one reply body.
	// https://www.interactivebrokers.com/docs/web-api/v1/endpoints/orders/place-order-reply-confirmation
	require.Equal(t, `{"confirmed":true}`, string(replies[0].body))
	require.NotContains(t, replies[0].path, ibkrSecondID)
}

func TestIBKRSecondPromptRejected(t *testing.T) {
	b := &ibkrBroker{placeBodies: []string{ibkrPrompt}, replyBodies: []string{ibkrSecondPrompt}}
	client, _ := newIBKR(t, b)

	placed, err := client.PlaceOrder(context.Background(), marketOrder(10))
	require.ErrorIs(t, err, supplier.ErrRejected)
	require.Nil(t, placed)
	require.Len(t, b.orderPosts(), 1)
	replies := replyCalls(b)
	require.Len(t, replies, 1)
	require.Equal(t, "/v1/api/iserver/reply/"+ibkrReplyID, replies[0].path)
	require.NotContains(t, replies[0].path, ibkrSecondID)
	for _, call := range b.calls {
		require.NotContains(t, call.path, ibkrSecondID)
	}
}

func TestIBKREmptyISIN(t *testing.T) {
	b := &ibkrBroker{}
	client, _ := newIBKR(t, b)
	order := marketOrder(10)
	order.Isin = "  "
	placed, err := client.PlaceOrder(context.Background(), order)
	require.ErrorIs(t, err, supplier.ErrNotTradable)
	require.Nil(t, placed)
	require.Empty(t, b.orderPosts())
}

func TestIBKREmptyBrokerID(t *testing.T) {
	book, err := trading.Load("")
	require.NoError(t, err)
	row, ok := book.Get("US1912161007")
	require.True(t, ok)
	require.Zero(t, row.IBKRConID)

	b := &ibkrBroker{}
	client, _ := newIBKR(t, b)
	order := marketOrder(10)
	order.Isin = row.ISIN
	placed, err := client.PlaceOrder(context.Background(), order)
	require.ErrorIs(t, err, supplier.ErrNotTradable)
	require.Nil(t, placed)
	require.Empty(t, b.orderPosts())
}

func TestIBKRListingConID(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "listings.json")
	raw := []byte(`[{
		"isin": "US0378331005",
		"symbol": "AAPL",
		"currency": "USD",
		"alpaca_symbol": "AAPL",
		"saxo_uic": 0,
		"saxo_asset_type": "",
		"ibkr_conid": 424242,
		"ig_epic": "",
		"ig_share_size": 0
	}]`)
	require.NoError(t, os.WriteFile(path, raw, 0o644))
	book, err := trading.Load(path)
	require.NoError(t, err)

	b := &ibkrBroker{}
	client := newIBKRBook(t, b, book, trading.IBKRConfig{})
	_, err = client.PlaceOrder(context.Background(), marketOrder(10))
	require.NoError(t, err)
	posts := b.orderPosts()
	require.Len(t, posts, 1)
	require.Contains(t, string(posts[0].body), `"conid":424242`)
	require.NotContains(t, string(posts[0].body), `"conid":265598`)
}

func TestIBKRPartialFill(t *testing.T) {
	b := &ibkrBroker{}
	client, book := newIBKR(t, b)
	row := aaplIBKR(t, book)
	// filledQuantity is the size already filled. avgPrice is the execution price.
	// https://www.interactivebrokers.com/docs/web-api/trading/orders/monitoring-live-orders
	b.ordersBody = fmt.Sprintf(`{"orders":[{
		"conid": %d,
		"orderId": 1234568790,
		"remainingQuantity": 6.0,
		"filledQuantity": 4.0,
		"totalSize": 10.0,
		"status": "Submitted",
		"orderType": "Market",
		"timeInForce": "DAY",
		"side": "SELL",
		"avgPrice": "192.26"
	}]}`, row.IBKRConID)

	got, err := client.GetOrders(context.Background())
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, pb.OrderStatus_ORDER_STATUS_PARTIALLY_FILLED, got[0].GetStatus())
	require.Equal(t, int32(-10), got[0].GetAmount())
	require.Equal(t, int32(-4), got[0].GetFilledAmount())
	require.InDelta(t, 192.26, got[0].GetAveragePrice(), 1e-9)
}

func TestIBKRBrokerHTTP401(t *testing.T) {
	b := &ibkrBroker{placeStatus: http.StatusUnauthorized}
	client, _ := newIBKR(t, b)
	placed, err := client.PlaceOrder(context.Background(), marketOrder(10))
	require.ErrorIs(t, err, supplier.ErrUnavailable)
	require.Nil(t, placed)
	require.Empty(t, replyCalls(b))
}

func TestIBKRLocalRejects(t *testing.T) {
	b := &ibkrBroker{}
	client, _ := newIBKR(t, b)

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

func TestIBKRShortPositionIsNegative(t *testing.T) {
	b := &ibkrBroker{accountBody: ibkrAccountUSD, ledgerBody: ibkrLedgerUSD}
	client, book := newIBKR(t, b)
	row, ok := book.Get(msftISIN)
	require.True(t, ok)
	// position is a signed size. A short is negative.
	// https://www.interactivebrokers.com/docs/web-api/v1/endpoints/portfolio/positions-new
	b.positionsBody = ibkrPositionBody(row.IBKRConID, -3, 90)

	got, err := client.GetPortfolio(context.Background())
	require.NoError(t, err)
	require.Len(t, got.GetPositions(), 1)
	require.Equal(t, row.ISIN, got.GetPositions()[0].GetIsin())
	require.Equal(t, row.Symbol, got.GetPositions()[0].GetSymbol())
	require.Equal(t, int32(-3), got.GetPositions()[0].GetAmount())
}

func TestIBKRPlaceErrorObject(t *testing.T) {
	// HTTP 200 with error is a rejection, not an order id.
	// https://www.interactivebrokers.com/docs/web-api/v1/endpoints/orders/place-order
	b := &ibkrBroker{placeBodies: []string{`{"error":"We cannot accept an order at the limit price you selected. Please submit your order using a limit price that is closer to the current market price of 197.79.  Alternatively, you can convert your order to an Algorithmic Order (IBALGO)."}`}}
	client, _ := newIBKR(t, b)
	placed, err := client.PlaceOrder(context.Background(), marketOrder(10))
	require.ErrorIs(t, err, supplier.ErrRejected)
	require.Nil(t, placed)
	require.Empty(t, replyCalls(b))
}

func TestIBKRGatewayDefault(t *testing.T) {
	book, err := trading.Load("")
	require.NoError(t, err)
	transport := &captureTransport{}
	client, err := trading.NewIBKR(trading.IBKRConfig{
		AccountMode: "paper",
		AccountID:   ibkrAccountID,
	}, book, &http.Client{Transport: transport})
	require.NoError(t, err)
	_, err = client.GetPortfolio(context.Background())
	require.ErrorIs(t, err, supplier.ErrUnavailable)
	require.NotNil(t, transport.req)
	require.Equal(t, "localhost:5000", transport.req.URL.Host)
	require.Equal(t, "/v1/api/portfolio/accounts", transport.req.URL.Path)
	require.Equal(t, "https", transport.req.URL.Scheme)
	require.Empty(t, transport.req.Header.Get("Authorization"))
}

func TestIBKRAccountMode(t *testing.T) {
	book, err := trading.Load("")
	require.NoError(t, err)
	client := &http.Client{}

	_, err = trading.NewIBKR(trading.IBKRConfig{AccountID: ibkrAccountID}, book, client)
	require.Error(t, err)
	_, err = trading.NewIBKR(trading.IBKRConfig{AccountMode: "  ", AccountID: ibkrAccountID}, book, client)
	require.Error(t, err)
	_, err = trading.NewIBKR(trading.IBKRConfig{AccountMode: "live", AccountID: ibkrAccountID}, book, client)
	require.Error(t, err)
	_, err = trading.NewIBKR(trading.IBKRConfig{AccountMode: "LIVE", AccountID: ibkrAccountID}, book, client)
	require.Error(t, err)
	_, err = trading.NewIBKR(trading.IBKRConfig{AccountMode: "demo", AccountID: ibkrAccountID}, book, client)
	require.Error(t, err)
	_, err = trading.NewIBKR(trading.IBKRConfig{AccountMode: "paper"}, book, client)
	require.Error(t, err)
	_, err = trading.NewIBKR(trading.IBKRConfig{AccountMode: "paper", AccountID: ibkrAccountID}, nil, client)
	require.Error(t, err)
	_, err = trading.NewIBKR(trading.IBKRConfig{AccountMode: "paper", AccountID: ibkrAccountID}, book, nil)
	require.Error(t, err)

	_, err = trading.NewIBKR(trading.IBKRConfig{
		AccountMode:  "live",
		AllowLiveURL: true,
		AccountID:    ibkrAccountID,
	}, book, client)
	require.NoError(t, err)
	_, err = trading.NewIBKR(trading.IBKRConfig{
		AccountMode: "paper",
		AccountID:   ibkrAccountID,
	}, book, client)
	require.NoError(t, err)
	_, err = trading.NewIBKR(trading.IBKRConfig{
		BaseURL:     "https://localhost:5000/v1/api",
		AccountMode: "paper",
		AccountID:   ibkrAccountID,
	}, book, client)
	require.NoError(t, err)
}

func TestIBKRMultiplePrompts(t *testing.T) {
	// Two prompts in one body are not confirmed.
	// https://www.interactivebrokers.com/docs/web-api/v1/endpoints/orders/place-order
	body := `[{
		"id": "07a13a5a-4a48-44a5-bb25-5ab37b79186c",
		"message": ["first"]
	},{
		"id": "a12b34c5-d678-9e012f-3456-7a890b12cd3e",
		"message": ["second"]
	}]`
	b := &ibkrBroker{placeBodies: []string{body}}
	client, _ := newIBKR(t, b)
	placed, err := client.PlaceOrder(context.Background(), marketOrder(10))
	require.ErrorIs(t, err, supplier.ErrRejected)
	require.Nil(t, placed)
	require.Empty(t, replyCalls(b))
}

func replyCalls(b *ibkrBroker) []recorded {
	b.mu.Lock()
	defer b.mu.Unlock()
	var out []recorded
	for _, call := range b.calls {
		if call.method == http.MethodPost && strings.Contains(call.path, "/iserver/reply/") {
			out = append(out, call)
		}
	}
	return out
}

func callLines(calls []recorded) []string {
	out := make([]string, 0, len(calls))
	for _, call := range calls {
		out = append(out, call.method+" "+call.path)
	}
	return out
}
