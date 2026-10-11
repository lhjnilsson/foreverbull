package trading

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"

	"github.com/lhjnilsson/foreverbull/pkg/finance/supplier"
	pb "github.com/lhjnilsson/foreverbull/pkg/pb/finance"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	// Local Client Portal gateway. This is not a hosted trading URL.
	// https://www.interactivebrokers.com/docs/web-api/api/web-api/placing-orders
	ibkrGatewayBaseURL = "https://localhost:5000/v1/api"

	// Account currency. Call this before other /portfolio reads.
	// https://www.interactivebrokers.com/docs/web-api/v1/endpoints/portfolio/portfolio-accounts
	ibkrAccountsPath = "/portfolio/accounts"
	// Cash and net liquidation for one currency. BASE is not an account currency.
	// https://www.interactivebrokers.com/docs/web-api/v1/endpoints/portfolio/portfolio-ledger
	ibkrLedgerPath = "/portfolio/%s/ledger"
	// Signed position size and conid.
	// https://www.interactivebrokers.com/docs/web-api/v1/endpoints/portfolio/positions-new
	ibkrPositionsPath = "/portfolio2/%s/positions"
	// One order ticket. https://www.interactivebrokers.com/docs/web-api/v1/endpoints/orders/place-order
	ibkrOrdersPath = "/iserver/account/%s/orders"
	// One confirmation. A second prompt is not posted.
	// https://www.interactivebrokers.com/docs/web-api/v1/endpoints/orders/place-order-reply-confirmation
	ibkrReplyPath = "/iserver/reply/%s"
	// Open orders. filters are the open status values. force is omitted: the
	// live-orders page says a force submission appears as a blank array.
	// https://www.interactivebrokers.com/docs/web-api/v1/endpoints/order-monitoring/live-orders
	// https://www.interactivebrokers.com/docs/web-api/v1/endpoints/order-monitoring/order-status-value
	ibkrLiveOrdersPath = "/iserver/account/orders"

	defaultIBKRCurrency = "USD"
	ibkrOpenFilters     = "pending_submit,pre_submitted,submitted"
)

// IBKRConfig is supplied by module.go. NewIBKR does not read the environment.
type IBKRConfig struct {
	// BaseURL is the gateway origin. Empty selects the local gateway.
	BaseURL string
	// AccountID is the account in the order and portfolio paths. Empty is rejected.
	AccountID string
	// AccountMode is paper or live. Empty is rejected. The factory sets paper.
	// live is refused unless AllowLiveURL is set. That flag does not unlock mode LIVE.
	AccountMode string
	// AccessToken is an optional bearer token. Empty is not sent.
	AccessToken string
	// Currency is compared to the account currency. Empty means USD.
	Currency string
	// AllowLiveURL permits AccountMode live. It does not unlock mode LIVE.
	AllowLiveURL bool
}

// IBKRClient is the Client Portal gateway trading HTTP adapter.
type IBKRClient struct {
	baseURL    string
	accountID  string
	token      string
	currency   string
	httpClient *http.Client
	byISIN     map[string]Listing
	byConID    map[int64]Listing
}

var _ supplier.Trading = (*IBKRClient)(nil)

// NewIBKR builds a trading client. book and client are required.
// AccountMode and AccountID are required. An empty mode is rejected here;
// the factory supplies the paper default. A nil book or client is an error.
func NewIBKR(cfg IBKRConfig, book *Book, client *http.Client) (*IBKRClient, error) {
	if book == nil {
		return nil, errors.New("ibkr trading: nil listings")
	}
	if client == nil {
		return nil, errors.New("ibkr trading: nil http client")
	}
	if err := ibkrAccountMode(cfg); err != nil {
		return nil, err
	}
	accountID := strings.TrimSpace(cfg.AccountID)
	if accountID == "" {
		return nil, errors.New("ibkr trading: account id is empty")
	}
	if !validIBKRAccountID(accountID) {
		return nil, errors.New("ibkr trading: account id")
	}
	base, err := ibkrBaseURL(cfg.BaseURL)
	if err != nil {
		return nil, err
	}
	currency := strings.ToUpper(strings.TrimSpace(cfg.Currency))
	if currency == "" {
		currency = defaultIBKRCurrency
	}

	byISIN := make(map[string]Listing)
	byConID := make(map[int64]Listing)
	ambiguous := make(map[int64]struct{})
	for _, row := range book.List() {
		byISIN[row.ISIN] = row
		if row.IBKRConID == 0 {
			continue
		}
		if _, dup := ambiguous[row.IBKRConID]; dup {
			continue
		}
		if _, seen := byConID[row.IBKRConID]; seen {
			delete(byConID, row.IBKRConID)
			ambiguous[row.IBKRConID] = struct{}{}
			continue
		}
		byConID[row.IBKRConID] = row
	}

	return &IBKRClient{
		baseURL:    base,
		accountID:  accountID,
		token:      strings.TrimSpace(cfg.AccessToken),
		currency:   currency,
		httpClient: client,
		byISIN:     byISIN,
		byConID:    byConID,
	}, nil
}

func ibkrAccountMode(cfg IBKRConfig) error {
	switch strings.ToLower(strings.TrimSpace(cfg.AccountMode)) {
	case "":
		return errors.New("ibkr trading: account mode is empty")
	case "paper":
		return nil
	case "live":
		if !cfg.AllowLiveURL {
			return errors.New("ibkr trading: live account mode refused")
		}
		return nil
	default:
		return fmt.Errorf("ibkr trading: account mode %q", cfg.AccountMode)
	}
}

func validIBKRAccountID(id string) bool {
	if id == "" || len(id) > 64 {
		return false
	}
	for _, r := range id {
		if r <= ' ' || r == '/' || r == '\\' || r == '?' || r == '#' || r == '%' {
			return false
		}
	}
	return true
}

func ibkrBaseURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		raw = ibkrGatewayBaseURL
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("ibkr trading: base url %q", raw)
	}
	if parsed.Scheme != "https" && parsed.Scheme != "http" {
		return "", fmt.Errorf("ibkr trading: base url %q", raw)
	}
	if parsed.User != nil {
		return "", errors.New("ibkr trading: base url has userinfo")
	}
	parsed.Path = path.Clean(parsed.Path)
	if parsed.Path == "." || parsed.Path == "/" {
		parsed.Path = ""
	}
	parsed.RawPath = ""
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return strings.TrimRight(parsed.String(), "/"), nil
}

// GetPortfolio reads the account currency, the ledger for that currency, and positions.
func (c *IBKRClient) GetPortfolio(ctx context.Context) (*pb.Portfolio, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	accountBody, err := c.get(ctx, ibkrAccountsPath)
	if err != nil {
		return nil, err
	}
	var accounts []ibkrAccount
	if err := json.Unmarshal(accountBody, &accounts); err != nil {
		return nil, fmt.Errorf("ibkr accounts: %w", supplier.ErrUnavailable)
	}
	currency, err := c.accountCurrency(accounts)
	if err != nil {
		return nil, err
	}
	if currency != c.currency {
		return nil, fmt.Errorf("account currency %q: %w", currency, supplier.ErrCurrency)
	}

	ledgerBody, err := c.get(ctx, fmt.Sprintf(ibkrLedgerPath, url.PathEscape(c.accountID)))
	if err != nil {
		return nil, err
	}
	var ledger map[string]ibkrLedger
	if err := json.Unmarshal(ledgerBody, &ledger); err != nil {
		return nil, fmt.Errorf("ibkr ledger: %w", supplier.ErrUnavailable)
	}
	entry, ok := ledger[currency]
	if !ok {
		return nil, fmt.Errorf("ibkr ledger %s: %w", currency, supplier.ErrUnavailable)
	}
	gotCurrency := strings.ToUpper(strings.TrimSpace(entry.Currency))
	if gotCurrency != "" && gotCurrency != currency {
		return nil, fmt.Errorf("ledger currency %q: %w", entry.Currency, supplier.ErrCurrency)
	}
	if !moneyOK(entry.CashBalance) || !moneyOK(entry.NetLiquidation) {
		return nil, fmt.Errorf("ibkr ledger: %w", supplier.ErrUnavailable)
	}

	positionBody, err := c.get(ctx, fmt.Sprintf(ibkrPositionsPath, url.PathEscape(c.accountID)))
	if err != nil {
		return nil, err
	}
	var rawPositions []ibkrPosition
	if err := json.Unmarshal(positionBody, &rawPositions); err != nil {
		return nil, fmt.Errorf("ibkr positions: %w", supplier.ErrUnavailable)
	}
	positions := make([]*pb.Position, 0, len(rawPositions))
	for _, raw := range rawPositions {
		pos, err := c.mapPosition(raw)
		if err != nil {
			return nil, err
		}
		positions = append(positions, pos)
	}

	return &pb.Portfolio{
		Timestamp:      timestamppb.Now(),
		Cash:           entry.CashBalance,
		PortfolioValue: entry.NetLiquidation,
		Positions:      positions,
	}, nil
}

// PlaceOrder posts one day order. A signed amount becomes BUY or SELL and a
// positive quantity. conid comes from the listing. One confirmation prompt is
// posted once. A second prompt returns ErrRejected and does not loop.
func (c *IBKRClient) PlaceOrder(ctx context.Context, order *pb.Order) (*pb.Order, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	listing, req, err := c.orderRequest(order)
	if err != nil {
		return nil, err
	}
	payload, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("ibkr order: %w", err)
	}
	body, err := c.post(ctx, fmt.Sprintf(ibkrOrdersPath, url.PathEscape(c.accountID)), payload)
	if err != nil {
		return nil, err
	}
	return c.finishPlace(ctx, listing, order, body, true)
}

// GetOrders reads open orders. An empty slice is returned only when orders is empty.
func (c *IBKRClient) GetOrders(ctx context.Context) ([]*pb.Order, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	query := url.Values{}
	query.Set("accountId", c.accountID)
	query.Set("filters", ibkrOpenFilters)
	body, err := c.get(ctx, ibkrLiveOrdersPath+"?"+query.Encode())
	if err != nil {
		return nil, err
	}
	var page ibkrLivePage
	if err := json.Unmarshal(body, &page); err != nil {
		return nil, fmt.Errorf("ibkr orders: %w", supplier.ErrUnavailable)
	}
	out := make([]*pb.Order, 0, len(page.Orders))
	for _, raw := range page.Orders {
		listing, ok := c.byConID[int64(raw.Conid)]
		if !ok || listing.ISIN == "" {
			return nil, fmt.Errorf("order conid %d: %w", raw.Conid, supplier.ErrNotTradable)
		}
		mapped, err := mapIBKROrder(raw, listing)
		if err != nil {
			return nil, err
		}
		out = append(out, mapped)
	}
	return out, nil
}

func (c *IBKRClient) accountCurrency(accounts []ibkrAccount) (string, error) {
	if len(accounts) == 0 {
		return "", fmt.Errorf("ibkr accounts empty: %w", supplier.ErrUnavailable)
	}
	for _, account := range accounts {
		id := strings.TrimSpace(account.ID)
		alt := strings.TrimSpace(account.AccountID)
		if id != c.accountID && alt != c.accountID {
			continue
		}
		currency := strings.ToUpper(strings.TrimSpace(account.Currency))
		if currency == "" {
			return "", fmt.Errorf("ibkr account currency: %w", supplier.ErrUnavailable)
		}
		return currency, nil
	}
	return "", fmt.Errorf("ibkr account: %w", supplier.ErrUnavailable)
}

// ibkrOrderTicket is one element of orders on POST /iserver/account/{accountId}/orders.
// Market orders omit price. orderType MKT is the market ticket; LMT sends price.
// https://www.interactivebrokers.com/docs/web-api/api/web-api/placing-orders
// https://www.interactivebrokers.com/docs/web-api/v1/endpoints/orders/place-order
// https://www.interactivebrokers.com/docs/web-api/v1/endpoints/orders/bracket-orders-oca-groups
type ibkrOrderTicket struct {
	Conid     int64    `json:"conid"`
	OrderType string   `json:"orderType"`
	Price     *float64 `json:"price,omitempty"`
	Quantity  int32    `json:"quantity"`
	Side      string   `json:"side"`
	Tif       string   `json:"tif"`
}

type ibkrPlaceRequest struct {
	Orders []ibkrOrderTicket `json:"orders"`
}

// ibkrConfirm is the single reply body.
// https://www.interactivebrokers.com/docs/web-api/v1/endpoints/orders/place-order-reply-confirmation
type ibkrConfirm struct {
	Confirmed bool `json:"confirmed"`
}

func (c *IBKRClient) orderRequest(order *pb.Order) (Listing, ibkrPlaceRequest, error) {
	if order == nil {
		return Listing{}, ibkrPlaceRequest{}, fmt.Errorf("nil order: %w", supplier.ErrRejected)
	}
	isin := normalizeISIN(order.GetIsin())
	if isin == "" {
		return Listing{}, ibkrPlaceRequest{}, fmt.Errorf("empty isin: %w", supplier.ErrNotTradable)
	}
	listing, ok := c.byISIN[isin]
	if !ok || listing.IBKRConID == 0 {
		return Listing{}, ibkrPlaceRequest{}, fmt.Errorf("isin %s: %w", isin, supplier.ErrNotTradable)
	}
	amount := order.GetAmount()
	if amount == 0 || amount == math.MinInt32 {
		return Listing{}, ibkrPlaceRequest{}, fmt.Errorf("amount: %w", supplier.ErrRejected)
	}
	side := "BUY"
	qty := amount
	if qty < 0 {
		side = "SELL"
		qty = -qty
	}

	var orderType string
	switch order.GetType() {
	case pb.OrderType_ORDER_TYPE_MARKET:
		orderType = "MKT"
	case pb.OrderType_ORDER_TYPE_LIMIT:
		orderType = "LMT"
	case pb.OrderType_ORDER_TYPE_UNSPECIFIED:
		return Listing{}, ibkrPlaceRequest{}, fmt.Errorf("order type %s: %w", order.GetType(), supplier.ErrRejected)
	default:
		return Listing{}, ibkrPlaceRequest{}, fmt.Errorf("order type %s: %w", order.GetType(), supplier.ErrRejected)
	}
	switch order.GetTimeInForce() {
	case pb.TimeInForce_TIME_IN_FORCE_UNSPECIFIED, pb.TimeInForce_TIME_IN_FORCE_DAY:
	default:
		return Listing{}, ibkrPlaceRequest{}, fmt.Errorf("time in force %s: %w", order.GetTimeInForce(), supplier.ErrRejected)
	}

	ticket := ibkrOrderTicket{
		Conid:     listing.IBKRConID,
		OrderType: orderType,
		Quantity:  qty,
		Side:      side,
		Tif:       "DAY",
	}
	if orderType == "LMT" {
		price := order.GetLimitPrice()
		if price <= 0 || math.IsNaN(price) || math.IsInf(price, 0) {
			return Listing{}, ibkrPlaceRequest{}, fmt.Errorf("limit price: %w", supplier.ErrRejected)
		}
		ticket.Price = &price
	}
	return listing, ibkrPlaceRequest{Orders: []ibkrOrderTicket{ticket}}, nil
}

func (c *IBKRClient) finishPlace(ctx context.Context, listing Listing, order *pb.Order, body []byte, allowReply bool) (*pb.Order, error) {
	items, err := decodeIBKRResults(body)
	if err != nil {
		return nil, fmt.Errorf("ibkr order: %w", supplier.ErrUnavailable)
	}
	ack, prompt, err := classifyIBKR(items)
	if err != nil {
		return nil, err
	}
	if ack != nil {
		return acceptedIBKR(listing, order, string(ack.OrderID))
	}
	if prompt == nil {
		return nil, fmt.Errorf("ibkr order missing id: %w", supplier.ErrUnavailable)
	}
	if !allowReply {
		msg := clipIBKR(prompt.text())
		if msg == "" {
			return nil, fmt.Errorf("ibkr order confirmation: %w", supplier.ErrRejected)
		}
		return nil, fmt.Errorf("ibkr order confirmation: %s: %w", msg, supplier.ErrRejected)
	}
	replyPath, err := ibkrOneReply(prompt.ID)
	if err != nil {
		return nil, err
	}
	confirmed, err := json.Marshal(ibkrConfirm{Confirmed: true})
	if err != nil {
		return nil, fmt.Errorf("ibkr reply: %w", err)
	}
	next, err := c.post(ctx, replyPath, confirmed)
	if err != nil {
		return nil, err
	}
	return c.finishPlace(ctx, listing, order, next, false)
}

func (c *IBKRClient) mapPosition(raw ibkrPosition) (*pb.Position, error) {
	listing, ok := c.byConID[int64(raw.Conid)]
	if !ok || listing.ISIN == "" {
		return nil, fmt.Errorf("position conid %d: %w", raw.Conid, supplier.ErrUnmappedPosition)
	}
	qty, err := wholeCount(raw.Position)
	if err != nil {
		return nil, fmt.Errorf("position conid %d: %w", raw.Conid, supplier.ErrUnavailable)
	}
	pos := &pb.Position{
		Symbol: listing.Symbol,
		Isin:   listing.ISIN,
		Amount: qty,
	}
	if raw.AvgCost != nil {
		if !moneyOK(*raw.AvgCost) {
			return nil, fmt.Errorf("position conid %d cost: %w", raw.Conid, supplier.ErrUnavailable)
		}
		pos.CostBasis = *raw.AvgCost
	}
	return pos, nil
}

func mapIBKROrder(raw ibkrLiveOrder, listing Listing) (*pb.Order, error) {
	if strings.TrimSpace(string(raw.OrderID)) == "" {
		return nil, fmt.Errorf("ibkr order missing id: %w", supplier.ErrUnavailable)
	}
	sign, err := ibkrSideSign(raw.Side)
	if err != nil {
		return nil, err
	}
	qty, err := ibkrOrderQty(raw)
	if err != nil {
		return nil, err
	}
	filled, err := wholeCount(raw.FilledQuantity)
	if err != nil || filled < 0 {
		return nil, fmt.Errorf("ibkr filled: %w", supplier.ErrUnavailable)
	}
	kind := raw.OrderType
	if strings.TrimSpace(kind) == "" {
		kind = raw.OrigOrderType
	}
	orderType, err := ibkrOrderType(kind)
	if err != nil {
		return nil, err
	}
	out := &pb.Order{
		Symbol:        listing.Symbol,
		Isin:          listing.ISIN,
		Amount:        sign * qty,
		Type:          orderType,
		TimeInForce:   ibkrTIF(raw.TimeInForce),
		Status:        fillStatus(filled, qty),
		FilledAmount:  sign * filled,
		BrokerOrderId: string(raw.OrderID),
	}
	if filled != 0 {
		px, err := parseMoney(raw.AvgPrice)
		if err != nil {
			return nil, fmt.Errorf("ibkr average price: %w", supplier.ErrUnavailable)
		}
		out.AveragePrice = px
	}
	return out, nil
}

func acceptedIBKR(listing Listing, order *pb.Order, id string) (*pb.Order, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, fmt.Errorf("ibkr order missing id: %w", supplier.ErrUnavailable)
	}
	out := &pb.Order{
		Symbol:        listing.Symbol,
		Isin:          listing.ISIN,
		Amount:        order.GetAmount(),
		Type:          order.GetType(),
		TimeInForce:   pb.TimeInForce_TIME_IN_FORCE_DAY,
		Status:        pb.OrderStatus_ORDER_STATUS_ACCEPTED,
		BrokerOrderId: id,
	}
	if order.GetType() == pb.OrderType_ORDER_TYPE_LIMIT {
		out.LimitPrice = order.GetLimitPrice()
	}
	return out, nil
}

func ibkrOrderQty(raw ibkrLiveOrder) (int32, error) {
	if raw.TotalSize != 0 {
		qty, err := wholeCount(raw.TotalSize)
		if err != nil || qty <= 0 {
			return 0, fmt.Errorf("ibkr size: %w", supplier.ErrUnavailable)
		}
		return qty, nil
	}
	qty, err := wholeCount(raw.FilledQuantity + raw.RemainingQuantity)
	if err != nil || qty <= 0 {
		return 0, fmt.Errorf("ibkr size: %w", supplier.ErrUnavailable)
	}
	return qty, nil
}

func ibkrSideSign(side string) (int32, error) {
	switch strings.ToUpper(strings.TrimSpace(side)) {
	case "BUY":
		return 1, nil
	case "SELL":
		return -1, nil
	default:
		return 0, fmt.Errorf("ibkr side %q: %w", side, supplier.ErrUnavailable)
	}
}

func ibkrOrderType(raw string) (pb.OrderType, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "market", "mkt":
		return pb.OrderType_ORDER_TYPE_MARKET, nil
	case "limit", "lmt":
		return pb.OrderType_ORDER_TYPE_LIMIT, nil
	default:
		return pb.OrderType_ORDER_TYPE_UNSPECIFIED, fmt.Errorf("ibkr type %q: %w", raw, supplier.ErrUnavailable)
	}
}

func ibkrTIF(raw string) pb.TimeInForce {
	if strings.EqualFold(strings.TrimSpace(raw), "DAY") {
		return pb.TimeInForce_TIME_IN_FORCE_DAY
	}
	return pb.TimeInForce_TIME_IN_FORCE_UNSPECIFIED
}

func (c *IBKRClient) get(ctx context.Context, path string) ([]byte, error) {
	return c.exchange(ctx, http.MethodGet, path, nil)
}

func (c *IBKRClient) post(ctx context.Context, path string, payload []byte) ([]byte, error) {
	return c.exchange(ctx, http.MethodPost, path, payload)
}

func (c *IBKRClient) exchange(ctx context.Context, method, path string, payload []byte) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var body io.Reader
	if payload != nil {
		body = bytes.NewReader(payload)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return nil, fmt.Errorf("ibkr request: %w", err)
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	req.Header.Set("Accept", "application/json")
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("ibkr %s: %w", method, supplier.ErrUnavailable)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("ibkr read: %w", supplier.ErrUnavailable)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, ibkrHTTPError(resp.StatusCode, raw)
	}
	return raw, nil
}

func ibkrHTTPError(code int, body []byte) error {
	msg := ibkrMessage(body)
	switch {
	case code == http.StatusUnauthorized || code >= http.StatusInternalServerError:
		if msg == "" {
			return fmt.Errorf("ibkr http %d: %w", code, supplier.ErrUnavailable)
		}
		return fmt.Errorf("ibkr http %d: %s: %w", code, msg, supplier.ErrUnavailable)
	case code >= http.StatusBadRequest:
		if msg == "" {
			return fmt.Errorf("ibkr http %d: %w", code, supplier.ErrRejected)
		}
		return fmt.Errorf("ibkr http %d: %s: %w", code, msg, supplier.ErrRejected)
	default:
		return fmt.Errorf("ibkr http %d: %w", code, supplier.ErrUnavailable)
	}
}

func ibkrMessage(body []byte) string {
	var payload struct {
		Error   string `json:"error"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return ""
	}
	msg := strings.TrimSpace(payload.Error)
	if msg == "" {
		msg = strings.TrimSpace(payload.Message)
	}
	return clipIBKR(msg)
}

func clipIBKR(msg string) string {
	msg = strings.TrimSpace(msg)
	if len(msg) > 512 {
		return msg[:512]
	}
	return msg
}

func ibkrOneReply(id string) (string, error) {
	id = strings.TrimSpace(id)
	if id == "" || len(id) > 80 {
		return "", fmt.Errorf("ibkr reply id: %w", supplier.ErrRejected)
	}
	for _, r := range id {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-':
		default:
			return "", fmt.Errorf("ibkr reply id: %w", supplier.ErrRejected)
		}
	}
	return fmt.Sprintf(ibkrReplyPath, id), nil
}

func decodeIBKRResults(body []byte) ([]ibkrResultItem, error) {
	body = bytes.TrimSpace(body)
	if len(body) == 0 {
		return nil, errors.New("empty body")
	}
	switch body[0] {
	case '[':
		var items []ibkrResultItem
		if err := json.Unmarshal(body, &items); err != nil {
			return nil, err
		}
		return items, nil
	case '{':
		var item ibkrResultItem
		if err := json.Unmarshal(body, &item); err != nil {
			return nil, err
		}
		return []ibkrResultItem{item}, nil
	default:
		return nil, errors.New("ibkr order body")
	}
}

func classifyIBKR(items []ibkrResultItem) (*ibkrResultItem, *ibkrResultItem, error) {
	if len(items) == 0 {
		return nil, nil, fmt.Errorf("ibkr order missing id: %w", supplier.ErrUnavailable)
	}
	var ack *ibkrResultItem
	var prompts []ibkrResultItem
	for i := range items {
		item := items[i]
		if msg := strings.TrimSpace(item.Error); msg != "" {
			return nil, nil, fmt.Errorf("ibkr order rejected: %s: %w", clipIBKR(msg), supplier.ErrRejected)
		}
		if strings.TrimSpace(string(item.OrderID)) != "" {
			if ack == nil {
				ack = &items[i]
			}
			continue
		}
		if strings.TrimSpace(item.ID) != "" {
			prompts = append(prompts, item)
		}
	}
	if ack != nil {
		if ibkrPlaceRejected(ack.OrderStatus) {
			return nil, nil, fmt.Errorf("ibkr order rejected: %s: %w", ack.OrderStatus, supplier.ErrRejected)
		}
		return ack, nil, nil
	}
	if len(prompts) == 1 {
		return nil, &prompts[0], nil
	}
	if len(prompts) > 1 {
		return nil, nil, fmt.Errorf("ibkr order confirmation: %w", supplier.ErrRejected)
	}
	return nil, nil, fmt.Errorf("ibkr order missing id: %w", supplier.ErrUnavailable)
}

func ibkrPlaceRejected(status string) bool {
	switch strings.ToLower(strings.ReplaceAll(strings.TrimSpace(status), "_", "")) {
	case "cancelled", "canceled", "apicancelled", "inactive", "pendingcancel", "precancelled":
		return true
	default:
		return false
	}
}

type ibkrAccount struct {
	ID        string `json:"id"`
	AccountID string `json:"accountId"`
	Currency  string `json:"currency"`
}

type ibkrLedger struct {
	CashBalance    float64 `json:"cashbalance"`
	NetLiquidation float64 `json:"netliquidationvalue"`
	Currency       string  `json:"currency"`
}

type ibkrPosition struct {
	Conid    ibkrConid `json:"conid"`
	Position float64   `json:"position"`
	AvgCost  *float64  `json:"avgCost"`
}

type ibkrLivePage struct {
	Orders []ibkrLiveOrder `json:"orders"`
}

type ibkrLiveOrder struct {
	Conid             ibkrConid `json:"conid"`
	OrderID           ibkrID    `json:"orderId"`
	RemainingQuantity float64   `json:"remainingQuantity"`
	FilledQuantity    float64   `json:"filledQuantity"`
	TotalSize         float64   `json:"totalSize"`
	Status            string    `json:"status"`
	OrderType         string    `json:"orderType"`
	OrigOrderType     string    `json:"origOrderType"`
	TimeInForce       string    `json:"timeInForce"`
	Side              string    `json:"side"`
	AvgPrice          string    `json:"avgPrice"`
}

type ibkrResultItem struct {
	ID          string   `json:"id"`
	Message     []string `json:"message"`
	OrderID     ibkrID   `json:"order_id"`
	OrderStatus string   `json:"order_status"`
	Error       string   `json:"error"`
}

func (r ibkrResultItem) text() string {
	if len(r.Message) == 0 {
		return ""
	}
	return r.Message[0]
}

// ibkrID accepts the string order_id on place and the numeric orderId on live orders.
type ibkrID string

func (s *ibkrID) UnmarshalJSON(raw []byte) error {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || string(raw) == "null" {
		*s = ""
		return nil
	}
	if raw[0] == '"' {
		var value string
		if err := json.Unmarshal(raw, &value); err != nil {
			return err
		}
		*s = ibkrID(strings.TrimSpace(value))
		return nil
	}
	var number json.Number
	if err := json.Unmarshal(raw, &number); err != nil {
		return err
	}
	*s = ibkrID(number.String())
	return nil
}

// ibkrConid accepts the numeric conid and the string form in the positions example.
type ibkrConid int64

func (c *ibkrConid) UnmarshalJSON(raw []byte) error {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || string(raw) == "null" {
		*c = 0
		return nil
	}
	if raw[0] == '"' {
		var value string
		if err := json.Unmarshal(raw, &value); err != nil {
			return err
		}
		value = strings.TrimSpace(value)
		if value == "" {
			*c = 0
			return nil
		}
		n, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return err
		}
		*c = ibkrConid(n)
		return nil
	}
	var number json.Number
	if err := json.Unmarshal(raw, &number); err != nil {
		return err
	}
	n, err := number.Int64()
	if err != nil {
		return err
	}
	*c = ibkrConid(n)
	return nil
}
