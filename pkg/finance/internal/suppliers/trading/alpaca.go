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
	"strconv"
	"strings"

	"github.com/lhjnilsson/foreverbull/pkg/finance/supplier"
	pb "github.com/lhjnilsson/foreverbull/pkg/pb/finance"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	// Paper trading host. Live host api.alpaca.markets is refused unless
	// AlpacaConfig.AllowLiveURL is set. That flag does not unlock mode LIVE.
	alpacaPaperBaseURL = "https://paper-api.alpaca.markets"
	alpacaLiveHost     = "api.alpaca.markets"

	alpacaOrdersPath    = "/v2/orders"
	alpacaAccountPath   = "/v2/account"
	alpacaPositionsPath = "/v2/positions"

	// Documented maximum for GET /v2/orders. One request, no paging.
	alpacaOpenOrderLimit = "500"

	defaultAlpacaCurrency = "USD"
)

// AlpacaConfig is supplied by module.go. NewAlpaca does not read the environment.
type AlpacaConfig struct {
	// BaseURL is the trading origin. Empty selects the paper host.
	BaseURL string
	// APIKey is sent as APCA-API-KEY-ID. Empty stays empty.
	APIKey string
	// APISecret is sent as APCA-API-SECRET-KEY. Empty stays empty.
	APISecret string
	// Currency is compared to the account currency. Empty means USD.
	Currency string
	// AllowLiveURL permits host api.alpaca.markets. It does not unlock mode LIVE.
	AllowLiveURL bool
}

// AlpacaClient is the Alpaca trading HTTP adapter.
type AlpacaClient struct {
	baseURL    string
	apiKey     string
	apiSecret  string
	currency   string
	httpClient *http.Client
	byISIN     map[string]Listing
	bySymbol   map[string]symbolListing
}

type symbolListing struct {
	listing   Listing
	ambiguous bool
}

var _ supplier.Trading = (*AlpacaClient)(nil)

// NewAlpaca builds a trading client. book and client are required.
// A nil book or client is an error. The paper host is the default base URL.
func NewAlpaca(cfg AlpacaConfig, book *Book, client *http.Client) (*AlpacaClient, error) {
	if book == nil {
		return nil, errors.New("alpaca trading: nil listings")
	}
	if client == nil {
		return nil, errors.New("alpaca trading: nil http client")
	}

	base, err := alpacaBaseURL(cfg)
	if err != nil {
		return nil, err
	}
	currency := strings.ToUpper(strings.TrimSpace(cfg.Currency))
	if currency == "" {
		currency = defaultAlpacaCurrency
	}

	byISIN := make(map[string]Listing)
	bySymbol := make(map[string]symbolListing)
	for _, row := range book.List() {
		byISIN[row.ISIN] = row
		if row.AlpacaSymbol == "" {
			continue
		}
		prev, seen := bySymbol[row.AlpacaSymbol]
		if seen {
			prev.ambiguous = true
			bySymbol[row.AlpacaSymbol] = prev
			continue
		}
		bySymbol[row.AlpacaSymbol] = symbolListing{listing: row}
	}

	return &AlpacaClient{
		baseURL:    base,
		apiKey:     cfg.APIKey,
		apiSecret:  cfg.APISecret,
		currency:   currency,
		httpClient: client,
		byISIN:     byISIN,
		bySymbol:   bySymbol,
	}, nil
}

func alpacaBaseURL(cfg AlpacaConfig) (string, error) {
	raw := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if raw == "" {
		raw = alpacaPaperBaseURL
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("alpaca trading: base url %q", cfg.BaseURL)
	}
	if strings.EqualFold(parsed.Hostname(), alpacaLiveHost) && !cfg.AllowLiveURL {
		return "", fmt.Errorf("alpaca trading: live host %s refused", parsed.Hostname())
	}
	return raw, nil
}

// GetPortfolio reads GET /v2/account and GET /v2/positions.
func (c *AlpacaClient) GetPortfolio(ctx context.Context) (*pb.Portfolio, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	accountBody, err := c.get(ctx, alpacaAccountPath)
	if err != nil {
		return nil, err
	}
	var account alpacaAccount
	if err := json.Unmarshal(accountBody, &account); err != nil {
		return nil, fmt.Errorf("alpaca account: %w", supplier.ErrUnavailable)
	}
	gotCurrency := strings.ToUpper(strings.TrimSpace(account.Currency))
	if gotCurrency != c.currency {
		return nil, fmt.Errorf("account currency %q: %w", account.Currency, supplier.ErrCurrency)
	}
	cash, err := parseMoney(account.Cash)
	if err != nil {
		return nil, fmt.Errorf("alpaca cash: %w", supplier.ErrUnavailable)
	}
	equityRaw := strings.TrimSpace(account.Equity)
	if equityRaw == "" {
		equityRaw = account.PortfolioValue
	}
	equity, err := parseMoney(equityRaw)
	if err != nil {
		return nil, fmt.Errorf("alpaca equity: %w", supplier.ErrUnavailable)
	}

	positionBody, err := c.get(ctx, alpacaPositionsPath)
	if err != nil {
		return nil, err
	}
	var rawPositions []alpacaPosition
	if err := json.Unmarshal(positionBody, &rawPositions); err != nil {
		return nil, fmt.Errorf("alpaca positions: %w", supplier.ErrUnavailable)
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
		Cash:           cash,
		PortfolioValue: equity,
		Positions:      positions,
	}, nil
}

// PlaceOrder posts one day order. A signed amount becomes buy or sell and a
// positive qty. position_intent is not sent. The first error is returned as-is.
func (c *AlpacaClient) PlaceOrder(ctx context.Context, order *pb.Order) (*pb.Order, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	listing, req, err := c.orderRequest(order)
	if err != nil {
		return nil, err
	}
	payload, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("alpaca order: %w", err)
	}
	body, err := c.post(ctx, alpacaOrdersPath, payload)
	if err != nil {
		return nil, err
	}
	var raw alpacaOrder
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("alpaca order: %w", supplier.ErrUnavailable)
	}
	placed, err := c.mapOrder(raw, listing, true)
	if err != nil {
		return nil, err
	}
	return placed, nil
}

// GetOrders reads open orders. An empty slice is returned only when the body is empty.
func (c *AlpacaClient) GetOrders(ctx context.Context) ([]*pb.Order, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	query := url.Values{}
	query.Set("status", "open")
	query.Set("limit", alpacaOpenOrderLimit)
	body, err := c.get(ctx, alpacaOrdersPath+"?"+query.Encode())
	if err != nil {
		return nil, err
	}
	var raws []alpacaOrder
	if err := json.Unmarshal(body, &raws); err != nil {
		return nil, fmt.Errorf("alpaca orders: %w", supplier.ErrUnavailable)
	}
	out := make([]*pb.Order, 0, len(raws))
	for _, raw := range raws {
		hit, ok := c.bySymbol[strings.TrimSpace(raw.Symbol)]
		if !ok || hit.ambiguous {
			return nil, fmt.Errorf("order %s: %w", raw.Symbol, supplier.ErrNotTradable)
		}
		mapped, err := c.mapOrder(raw, hit.listing, false)
		if err != nil {
			return nil, err
		}
		out = append(out, mapped)
	}
	return out, nil
}

// alpacaOrderRequest is POST /v2/orders. Field names match CreateOrderRequest.
// qty, limit_price, side, type, and time_in_force are strings. position_intent
// is intentionally absent.
type alpacaOrderRequest struct {
	Symbol      string `json:"symbol"`
	Qty         string `json:"qty"`
	Side        string `json:"side"`
	Type        string `json:"type"`
	TimeInForce string `json:"time_in_force"`
	LimitPrice  string `json:"limit_price,omitempty"`
}

func (c *AlpacaClient) orderRequest(order *pb.Order) (Listing, alpacaOrderRequest, error) {
	if order == nil {
		return Listing{}, alpacaOrderRequest{}, fmt.Errorf("nil order: %w", supplier.ErrRejected)
	}
	isin := normalizeISIN(order.GetIsin())
	if isin == "" {
		return Listing{}, alpacaOrderRequest{}, fmt.Errorf("empty isin: %w", supplier.ErrNotTradable)
	}
	listing, ok := c.byISIN[isin]
	if !ok || listing.AlpacaSymbol == "" {
		return Listing{}, alpacaOrderRequest{}, fmt.Errorf("isin %s: %w", isin, supplier.ErrNotTradable)
	}
	if order.GetAmount() == 0 {
		return Listing{}, alpacaOrderRequest{}, fmt.Errorf("amount is zero: %w", supplier.ErrRejected)
	}
	if order.GetAmount() == math.MinInt32 {
		return Listing{}, alpacaOrderRequest{}, fmt.Errorf("amount: %w", supplier.ErrRejected)
	}

	side := "buy"
	qty := order.GetAmount()
	if qty < 0 {
		side = "sell"
		qty = -qty
	}

	var orderType string
	switch order.GetType() {
	case pb.OrderType_ORDER_TYPE_MARKET:
		orderType = "market"
	case pb.OrderType_ORDER_TYPE_LIMIT:
		orderType = "limit"
	case pb.OrderType_ORDER_TYPE_UNSPECIFIED:
		return Listing{}, alpacaOrderRequest{}, fmt.Errorf("order type %s: %w", order.GetType(), supplier.ErrRejected)
	default:
		return Listing{}, alpacaOrderRequest{}, fmt.Errorf("order type %s: %w", order.GetType(), supplier.ErrRejected)
	}

	switch order.GetTimeInForce() {
	case pb.TimeInForce_TIME_IN_FORCE_UNSPECIFIED, pb.TimeInForce_TIME_IN_FORCE_DAY:
	default:
		return Listing{}, alpacaOrderRequest{}, fmt.Errorf("time in force %s: %w", order.GetTimeInForce(), supplier.ErrRejected)
	}

	req := alpacaOrderRequest{
		Symbol:      listing.AlpacaSymbol,
		Qty:         strconv.FormatInt(int64(qty), 10),
		Side:        side,
		Type:        orderType,
		TimeInForce: "day",
	}
	if orderType == "limit" {
		price := order.GetLimitPrice()
		if price <= 0 || math.IsNaN(price) || math.IsInf(price, 0) {
			return Listing{}, alpacaOrderRequest{}, fmt.Errorf("limit price: %w", supplier.ErrRejected)
		}
		req.LimitPrice = strconv.FormatFloat(price, 'f', -1, 64)
	}
	return listing, req, nil
}

func (c *AlpacaClient) mapPosition(raw alpacaPosition) (*pb.Position, error) {
	symbol := strings.TrimSpace(raw.Symbol)
	hit, ok := c.bySymbol[symbol]
	if !ok || hit.ambiguous || hit.listing.ISIN == "" {
		return nil, fmt.Errorf("position %s: %w", raw.Symbol, supplier.ErrUnmappedPosition)
	}
	qty, err := parseWholeShares(raw.Qty)
	if err != nil {
		return nil, fmt.Errorf("position %s: %w", raw.Symbol, err)
	}
	if qty < 0 {
		qty = -qty
	}
	switch strings.ToLower(strings.TrimSpace(raw.Side)) {
	case "long", "":
	case "short":
		qty = -qty
	default:
		return nil, fmt.Errorf("position %s side %s: %w", raw.Symbol, raw.Side, supplier.ErrUnmappedPosition)
	}
	var cost float64
	if strings.TrimSpace(raw.CostBasis) != "" {
		cost, err = parseMoney(raw.CostBasis)
		if err != nil {
			return nil, fmt.Errorf("position %s cost basis: %w", raw.Symbol, supplier.ErrUnavailable)
		}
	}
	return &pb.Position{
		Symbol:    hit.listing.Symbol,
		Isin:      hit.listing.ISIN,
		Amount:    qty,
		CostBasis: cost,
	}, nil
}

func (c *AlpacaClient) mapOrder(raw alpacaOrder, listing Listing, place bool) (*pb.Order, error) {
	if strings.TrimSpace(raw.ID) == "" {
		return nil, fmt.Errorf("alpaca order missing id: %w", supplier.ErrUnavailable)
	}
	if strings.EqualFold(strings.TrimSpace(raw.Status), "rejected") {
		if place {
			return nil, fmt.Errorf("alpaca order rejected: %w", supplier.ErrRejected)
		}
		return &pb.Order{
			Symbol:        listing.Symbol,
			Isin:          listing.ISIN,
			Status:        pb.OrderStatus_ORDER_STATUS_REJECTED,
			RejectReason:  "rejected",
			BrokerOrderId: raw.ID,
		}, nil
	}

	side, err := shareSign(raw.Side)
	if err != nil {
		return nil, err
	}
	qty, err := parseWholeShares(raw.qty())
	if err != nil {
		return nil, fmt.Errorf("alpaca qty: %w", supplier.ErrUnavailable)
	}
	if qty < 0 {
		qty = -qty
	}
	filled, err := parseWholeShares(raw.FilledQty)
	if err != nil {
		return nil, fmt.Errorf("alpaca filled qty: %w", supplier.ErrUnavailable)
	}
	if filled < 0 {
		filled = -filled
	}

	orderType, err := platformOrderType(raw.Type)
	if err != nil {
		return nil, err
	}
	status := fillStatus(filled, qty)
	out := &pb.Order{
		Symbol:        listing.Symbol,
		Isin:          listing.ISIN,
		Amount:        side * qty,
		Type:          orderType,
		TimeInForce:   platformTIF(raw.TimeInForce),
		Status:        status,
		FilledAmount:  side * filled,
		BrokerOrderId: raw.ID,
	}
	if orderType == pb.OrderType_ORDER_TYPE_LIMIT {
		if raw.LimitPrice != nil && strings.TrimSpace(*raw.LimitPrice) != "" {
			px, err := parseMoney(*raw.LimitPrice)
			if err != nil {
				return nil, fmt.Errorf("alpaca limit price: %w", supplier.ErrUnavailable)
			}
			out.LimitPrice = px
		}
	}
	if filled != 0 {
		if raw.FilledAvgPrice == nil || strings.TrimSpace(*raw.FilledAvgPrice) == "" {
			return nil, fmt.Errorf("alpaca filled order missing average price: %w", supplier.ErrUnavailable)
		}
		px, err := parseMoney(*raw.FilledAvgPrice)
		if err != nil {
			return nil, fmt.Errorf("alpaca average price: %w", supplier.ErrUnavailable)
		}
		out.AveragePrice = px
	}
	return out, nil
}

func (c *AlpacaClient) get(ctx context.Context, path string) ([]byte, error) {
	return c.exchange(ctx, http.MethodGet, path, nil)
}

func (c *AlpacaClient) post(ctx context.Context, path string, payload []byte) ([]byte, error) {
	return c.exchange(ctx, http.MethodPost, path, payload)
}

func (c *AlpacaClient) exchange(ctx context.Context, method, path string, payload []byte) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var body io.Reader
	if payload != nil {
		body = bytes.NewReader(payload)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return nil, fmt.Errorf("alpaca request: %w", err)
	}
	req.Header.Set("APCA-API-KEY-ID", c.apiKey)
	req.Header.Set("APCA-API-SECRET-KEY", c.apiSecret)
	req.Header.Set("Accept", "application/json")
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("alpaca %s: %w", method, supplier.ErrUnavailable)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("alpaca read: %w", supplier.ErrUnavailable)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, alpacaHTTPError(resp.StatusCode, raw)
	}
	return raw, nil
}

func alpacaHTTPError(code int, body []byte) error {
	msg := alpacaMessage(body)
	switch {
	case code == http.StatusUnauthorized || code >= http.StatusInternalServerError:
		if msg == "" {
			return fmt.Errorf("alpaca http %d: %w", code, supplier.ErrUnavailable)
		}
		return fmt.Errorf("alpaca http %d: %s: %w", code, msg, supplier.ErrUnavailable)
	case code >= http.StatusBadRequest:
		if msg == "" {
			return fmt.Errorf("alpaca http %d: %w", code, supplier.ErrRejected)
		}
		return fmt.Errorf("alpaca http %d: %s: %w", code, msg, supplier.ErrRejected)
	default:
		return fmt.Errorf("alpaca http %d: %w", code, supplier.ErrUnavailable)
	}
}

func alpacaMessage(body []byte) string {
	var payload struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return ""
	}
	msg := strings.TrimSpace(payload.Message)
	if len(msg) > 512 {
		return msg[:512]
	}
	return msg
}

type alpacaAccount struct {
	Cash           string `json:"cash"`
	PortfolioValue string `json:"portfolio_value"`
	Equity         string `json:"equity"`
	Currency       string `json:"currency"`
}

type alpacaPosition struct {
	Symbol    string `json:"symbol"`
	Qty       string `json:"qty"`
	Side      string `json:"side"`
	CostBasis string `json:"cost_basis"`
}

type alpacaOrder struct {
	ID             string  `json:"id"`
	Symbol         string  `json:"symbol"`
	Qty            *string `json:"qty"`
	FilledQty      string  `json:"filled_qty"`
	FilledAvgPrice *string `json:"filled_avg_price"`
	Side           string  `json:"side"`
	Type           string  `json:"type"`
	TimeInForce    string  `json:"time_in_force"`
	LimitPrice     *string `json:"limit_price"`
	Status         string  `json:"status"`
}

func (o alpacaOrder) qty() string {
	if o.Qty == nil {
		return ""
	}
	return *o.Qty
}

func shareSign(side string) (int32, error) {
	switch strings.ToLower(strings.TrimSpace(side)) {
	case "buy":
		return 1, nil
	case "sell":
		return -1, nil
	default:
		return 0, fmt.Errorf("alpaca side %q: %w", side, supplier.ErrUnavailable)
	}
}

func platformOrderType(raw string) (pb.OrderType, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "market":
		return pb.OrderType_ORDER_TYPE_MARKET, nil
	case "limit":
		return pb.OrderType_ORDER_TYPE_LIMIT, nil
	default:
		return pb.OrderType_ORDER_TYPE_UNSPECIFIED, fmt.Errorf("alpaca type %q: %w", raw, supplier.ErrUnavailable)
	}
}

func platformTIF(raw string) pb.TimeInForce {
	if strings.EqualFold(strings.TrimSpace(raw), "day") {
		return pb.TimeInForce_TIME_IN_FORCE_DAY
	}
	return pb.TimeInForce_TIME_IN_FORCE_UNSPECIFIED
}

func fillStatus(filled, qty int32) pb.OrderStatus {
	if filled == 0 {
		return pb.OrderStatus_ORDER_STATUS_ACCEPTED
	}
	if qty > 0 && filled < qty {
		return pb.OrderStatus_ORDER_STATUS_PARTIALLY_FILLED
	}
	return pb.OrderStatus_ORDER_STATUS_FILLED
}

func parseMoney(raw string) (float64, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, errors.New("empty amount")
	}
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, err
	}
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return 0, fmt.Errorf("invalid amount %q", raw)
	}
	return value, nil
}

func parseWholeShares(raw string) (int32, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, errors.New("empty share quantity")
	}
	sign := int32(1)
	switch raw[0] {
	case '+':
		raw = raw[1:]
	case '-':
		sign = -1
		raw = raw[1:]
	}
	if raw == "" {
		return 0, errors.New("empty share quantity")
	}
	whole, frac, hasFrac := strings.Cut(raw, ".")
	if whole == "" {
		whole = "0"
	}
	if strings.ContainsAny(whole, "+-eE") {
		return 0, fmt.Errorf("share quantity %q", raw)
	}
	if hasFrac {
		if frac == "" || strings.Trim(frac, "0") != "" {
			return 0, fmt.Errorf("share quantity %q is not a whole number", raw)
		}
		for _, r := range frac {
			if r < '0' || r > '9' {
				return 0, fmt.Errorf("share quantity %q", raw)
			}
		}
	}
	n, err := strconv.ParseInt(whole, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("share quantity %q: %w", raw, err)
	}
	if sign < 0 {
		return -int32(n), nil
	}
	return int32(n), nil
}
