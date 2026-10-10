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
	"strings"

	"github.com/lhjnilsson/foreverbull/pkg/finance/supplier"
	pb "github.com/lhjnilsson/foreverbull/pkg/pb/finance"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	// Simulation REST origin. Live https://gateway.saxobank.com/openapi has no
	// /sim/ segment and is refused unless AllowLiveURL is set. That flag does
	// not unlock mode LIVE.
	// https://www.developer.saxo/openapi/learn/environments
	saxoSimBaseURL = "https://gateway.saxobank.com/sim/openapi"

	// POST place order. https://www.developer.saxo/openapi/referencedocs/trade/v2/orders/post__trade
	saxoOrdersPath = "/trade/v2/orders"
	// Account currency. https://www.developer.saxo/openapi/referencedocs/port/v1/accounts/get__port__me
	saxoAccountsPath = "/port/v1/accounts/me"
	// CashBalance and TotalValue. https://www.developer.saxo/openapi/referencedocs/port/v1/balances/get__port__me
	saxoBalancesPath = "/port/v1/balances/me"
	// Net position amount and UIC. https://www.developer.saxo/openapi/referencedocs/port/v1/netpositions/get__port__me
	saxoNetPositionsPath = "/port/v1/netpositions/me"
	// Open orders. https://www.developer.saxo/openapi/referencedocs/port/v1/orders/get__port__me
	saxoOpenOrdersPath = "/port/v1/orders/me"

	defaultSaxoCurrency = "USD"
)

// SaxoConfig is supplied by module.go. NewSaxo does not read the environment.
type SaxoConfig struct {
	// BaseURL is the OpenAPI origin. Empty selects the simulation origin.
	BaseURL string
	// AccessToken is sent as Authorization: Bearer. Empty stays empty.
	AccessToken string
	// AccountKey is the recommended order account. Empty uses the client's default account.
	AccountKey string
	// Currency is compared to the account currency. Empty means USD.
	Currency string
	// AllowLiveURL permits a base path without /sim/. It does not unlock mode LIVE.
	AllowLiveURL bool
}

// SaxoClient is the Saxo OpenAPI trading HTTP adapter.
type SaxoClient struct {
	baseURL     string
	token       string
	accountKey  string
	currency    string
	httpClient  *http.Client
	byISIN      map[string]Listing
	byUIC       map[saxoKey]Listing
	uicListings map[int64][]Listing
}

type saxoKey struct {
	uic  int64
	kind string
}

var _ supplier.Trading = (*SaxoClient)(nil)

// NewSaxo builds a trading client. book and client are required.
// A nil book or client is an error. The simulation origin is the default base URL.
func NewSaxo(cfg SaxoConfig, book *Book, client *http.Client) (*SaxoClient, error) {
	if book == nil {
		return nil, errors.New("saxo trading: nil listings")
	}
	if client == nil {
		return nil, errors.New("saxo trading: nil http client")
	}

	base, err := saxoBaseURL(cfg)
	if err != nil {
		return nil, err
	}
	currency := strings.ToUpper(strings.TrimSpace(cfg.Currency))
	if currency == "" {
		currency = defaultSaxoCurrency
	}

	byISIN := make(map[string]Listing)
	byUIC := make(map[saxoKey]Listing)
	ambiguous := make(map[saxoKey]struct{})
	for _, row := range book.List() {
		byISIN[row.ISIN] = row
		if row.SaxoUIC == 0 || row.SaxoAssetType == "" {
			continue
		}
		key := saxoKey{uic: row.SaxoUIC, kind: strings.ToLower(row.SaxoAssetType)}
		if _, dup := ambiguous[key]; dup {
			continue
		}
		if _, seen := byUIC[key]; seen {
			delete(byUIC, key)
			ambiguous[key] = struct{}{}
			continue
		}
		byUIC[key] = row
	}
	uicListings := make(map[int64][]Listing)
	for key, row := range byUIC {
		uicListings[key.uic] = append(uicListings[key.uic], row)
	}

	return &SaxoClient{
		baseURL:     base,
		token:       cfg.AccessToken,
		accountKey:  strings.TrimSpace(cfg.AccountKey),
		currency:    currency,
		httpClient:  client,
		byISIN:      byISIN,
		byUIC:       byUIC,
		uicListings: uicListings,
	}, nil
}

func saxoBaseURL(cfg SaxoConfig) (string, error) {
	raw := strings.TrimSpace(cfg.BaseURL)
	if raw == "" {
		raw = saxoSimBaseURL
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("saxo trading: base url %q", cfg.BaseURL)
	}
	if parsed.Scheme != "https" && parsed.Scheme != "http" {
		return "", fmt.Errorf("saxo trading: base url %q", cfg.BaseURL)
	}
	if parsed.User != nil {
		return "", fmt.Errorf("saxo trading: base url %q", cfg.BaseURL)
	}
	parsed.Path = path.Clean(parsed.Path)
	if parsed.Path == "." || parsed.Path == "/" {
		parsed.Path = ""
	}
	parsed.RawPath = ""
	parsed.RawQuery = ""
	parsed.Fragment = ""
	if !strings.Contains(strings.ToLower(parsed.Path), "/sim/") && !cfg.AllowLiveURL {
		return "", fmt.Errorf("saxo trading: live url refused")
	}
	return strings.TrimRight(parsed.String(), "/"), nil
}

// GetPortfolio reads the account currency, client balance, and net positions.
func (c *SaxoClient) GetPortfolio(ctx context.Context) (*pb.Portfolio, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	accountBody, err := c.get(ctx, saxoAccountsPath)
	if err != nil {
		return nil, err
	}
	var accounts saxoAccountPage
	if err := json.Unmarshal(accountBody, &accounts); err != nil {
		return nil, fmt.Errorf("saxo accounts: %w", supplier.ErrUnavailable)
	}
	if strings.TrimSpace(accounts.Next) != "" {
		return nil, fmt.Errorf("saxo accounts page: %w", supplier.ErrUnavailable)
	}
	currency, err := c.accountCurrency(accounts.Data)
	if err != nil {
		return nil, err
	}
	if currency != c.currency {
		return nil, fmt.Errorf("account currency %q: %w", currency, supplier.ErrCurrency)
	}

	balanceBody, err := c.get(ctx, saxoBalancesPath)
	if err != nil {
		return nil, err
	}
	var balance saxoBalance
	if err := json.Unmarshal(balanceBody, &balance); err != nil {
		return nil, fmt.Errorf("saxo balances: %w", supplier.ErrUnavailable)
	}
	gotBalance := strings.ToUpper(strings.TrimSpace(balance.Currency))
	if gotBalance != "" && gotBalance != c.currency {
		return nil, fmt.Errorf("balance currency %q: %w", balance.Currency, supplier.ErrCurrency)
	}
	if !moneyOK(balance.CashBalance) || !moneyOK(balance.TotalValue) {
		return nil, fmt.Errorf("saxo balance: %w", supplier.ErrUnavailable)
	}

	positionBody, err := c.get(ctx, saxoNetPositionsPath)
	if err != nil {
		return nil, err
	}
	var positions saxoNetPositionPage
	if err := json.Unmarshal(positionBody, &positions); err != nil {
		return nil, fmt.Errorf("saxo positions: %w", supplier.ErrUnavailable)
	}
	if strings.TrimSpace(positions.Next) != "" {
		return nil, fmt.Errorf("saxo positions page: %w", supplier.ErrUnavailable)
	}
	mapped := make([]*pb.Position, 0, len(positions.Data))
	for _, raw := range positions.Data {
		pos, err := c.mapPosition(raw)
		if err != nil {
			return nil, err
		}
		mapped = append(mapped, pos)
	}

	return &pb.Portfolio{
		Timestamp:      timestamppb.Now(),
		Cash:           balance.CashBalance,
		PortfolioValue: balance.TotalValue,
		Positions:      mapped,
	}, nil
}

// PlaceOrder posts one day order. A signed amount becomes BuySell and a positive
// Amount. Uic and AssetType come from the listing. The first error is returned as-is.
func (c *SaxoClient) PlaceOrder(ctx context.Context, order *pb.Order) (*pb.Order, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	listing, req, err := c.orderRequest(order)
	if err != nil {
		return nil, err
	}
	payload, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("saxo order: %w", err)
	}
	body, err := c.post(ctx, saxoOrdersPath, payload)
	if err != nil {
		return nil, err
	}
	var placed saxoPlaceResponse
	if err := json.Unmarshal(body, &placed); err != nil {
		return nil, fmt.Errorf("saxo order: %w", supplier.ErrUnavailable)
	}
	if msg := placed.errorMessage(); msg != "" {
		return nil, fmt.Errorf("saxo order rejected: %s: %w", msg, supplier.ErrRejected)
	}
	if strings.TrimSpace(string(placed.OrderID)) == "" {
		return nil, fmt.Errorf("saxo order missing id: %w", supplier.ErrUnavailable)
	}
	out := &pb.Order{
		Symbol:        listing.Symbol,
		Isin:          listing.ISIN,
		Amount:        order.GetAmount(),
		Type:          order.GetType(),
		TimeInForce:   pb.TimeInForce_TIME_IN_FORCE_DAY,
		Status:        pb.OrderStatus_ORDER_STATUS_ACCEPTED,
		BrokerOrderId: string(placed.OrderID),
	}
	if order.GetType() == pb.OrderType_ORDER_TYPE_LIMIT {
		out.LimitPrice = order.GetLimitPrice()
	}
	return out, nil
}

// GetOrders reads open orders. An empty slice is returned only when Data is empty.
func (c *SaxoClient) GetOrders(ctx context.Context) ([]*pb.Order, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	query := url.Values{}
	query.Set("Status", "Working")
	body, err := c.get(ctx, saxoOpenOrdersPath+"?"+query.Encode())
	if err != nil {
		return nil, err
	}
	var page saxoOrderPage
	if err := json.Unmarshal(body, &page); err != nil {
		return nil, fmt.Errorf("saxo orders: %w", supplier.ErrUnavailable)
	}
	if strings.TrimSpace(page.Next) != "" {
		return nil, fmt.Errorf("saxo orders page: %w", supplier.ErrUnavailable)
	}
	out := make([]*pb.Order, 0, len(page.Data))
	for _, raw := range page.Data {
		listing, ok := c.listingFor(raw.Uic, raw.AssetType)
		if !ok {
			return nil, fmt.Errorf("order uic %d: %w", raw.Uic, supplier.ErrNotTradable)
		}
		mapped, err := mapSaxoOrder(raw, listing)
		if err != nil {
			return nil, err
		}
		out = append(out, mapped)
	}
	return out, nil
}

func (c *SaxoClient) accountCurrency(accounts []saxoAccount) (string, error) {
	if len(accounts) == 0 {
		return "", fmt.Errorf("saxo accounts empty: %w", supplier.ErrUnavailable)
	}
	selected := accounts
	if c.accountKey != "" {
		selected = nil
		for _, account := range accounts {
			if strings.TrimSpace(account.AccountKey) == c.accountKey {
				selected = []saxoAccount{account}
				break
			}
		}
		if len(selected) == 0 {
			return "", fmt.Errorf("saxo account: %w", supplier.ErrUnavailable)
		}
	}
	currency := ""
	for _, account := range selected {
		got := strings.ToUpper(strings.TrimSpace(account.Currency))
		if got == "" {
			return "", fmt.Errorf("saxo account currency: %w", supplier.ErrUnavailable)
		}
		if currency == "" {
			currency = got
			continue
		}
		if got != currency {
			return "", fmt.Errorf("account currency %q: %w", got, supplier.ErrCurrency)
		}
	}
	return currency, nil
}

func (c *SaxoClient) listingFor(uic int64, assetType string) (Listing, bool) {
	kind := strings.ToLower(strings.TrimSpace(assetType))
	if kind != "" {
		row, ok := c.byUIC[saxoKey{uic: uic, kind: kind}]
		return row, ok
	}
	rows := c.uicListings[uic]
	if len(rows) == 1 {
		return rows[0], true
	}
	return Listing{}, false
}

// saxoOrderRequest is POST /trade/v2/orders.
// Field names and DayOrder match the market and limit examples.
// ManualOrder false is an automatic order from strategy code.
// https://www.developer.saxo/openapi/learn/order-placement
type saxoOrderRequest struct {
	AccountKey    string            `json:"AccountKey,omitempty"`
	Amount        float64           `json:"Amount"`
	BuySell       string            `json:"BuySell"`
	OrderType     string            `json:"OrderType"`
	OrderPrice    *float64          `json:"OrderPrice,omitempty"`
	ManualOrder   bool              `json:"ManualOrder"`
	Uic           int64             `json:"Uic"`
	AssetType     string            `json:"AssetType"`
	OrderDuration saxoOrderDuration `json:"OrderDuration"`
}

type saxoOrderDuration struct {
	DurationType string `json:"DurationType"`
}

func (c *SaxoClient) orderRequest(order *pb.Order) (Listing, saxoOrderRequest, error) {
	if order == nil {
		return Listing{}, saxoOrderRequest{}, fmt.Errorf("nil order: %w", supplier.ErrRejected)
	}
	isin := normalizeISIN(order.GetIsin())
	if isin == "" {
		return Listing{}, saxoOrderRequest{}, fmt.Errorf("empty isin: %w", supplier.ErrNotTradable)
	}
	listing, ok := c.byISIN[isin]
	if !ok || listing.SaxoUIC == 0 || listing.SaxoAssetType == "" {
		return Listing{}, saxoOrderRequest{}, fmt.Errorf("isin %s: %w", isin, supplier.ErrNotTradable)
	}
	amount := order.GetAmount()
	if amount == 0 || amount == math.MinInt32 {
		return Listing{}, saxoOrderRequest{}, fmt.Errorf("amount: %w", supplier.ErrRejected)
	}
	side := "Buy"
	qty := amount
	if qty < 0 {
		side = "Sell"
		qty = -qty
	}

	var orderType string
	switch order.GetType() {
	case pb.OrderType_ORDER_TYPE_MARKET:
		orderType = "Market"
	case pb.OrderType_ORDER_TYPE_LIMIT:
		orderType = "Limit"
	case pb.OrderType_ORDER_TYPE_UNSPECIFIED:
		return Listing{}, saxoOrderRequest{}, fmt.Errorf("order type %s: %w", order.GetType(), supplier.ErrRejected)
	default:
		return Listing{}, saxoOrderRequest{}, fmt.Errorf("order type %s: %w", order.GetType(), supplier.ErrRejected)
	}
	switch order.GetTimeInForce() {
	case pb.TimeInForce_TIME_IN_FORCE_UNSPECIFIED, pb.TimeInForce_TIME_IN_FORCE_DAY:
	default:
		return Listing{}, saxoOrderRequest{}, fmt.Errorf("time in force %s: %w", order.GetTimeInForce(), supplier.ErrRejected)
	}

	req := saxoOrderRequest{
		AccountKey:  c.accountKey,
		Amount:      float64(qty),
		BuySell:     side,
		OrderType:   orderType,
		ManualOrder: false,
		Uic:         listing.SaxoUIC,
		AssetType:   listing.SaxoAssetType,
		OrderDuration: saxoOrderDuration{
			DurationType: "DayOrder",
		},
	}
	if orderType == "Limit" {
		price := order.GetLimitPrice()
		if price <= 0 || math.IsNaN(price) || math.IsInf(price, 0) {
			return Listing{}, saxoOrderRequest{}, fmt.Errorf("limit price: %w", supplier.ErrRejected)
		}
		req.OrderPrice = &price
	}
	return listing, req, nil
}

func (c *SaxoClient) mapPosition(raw saxoNetPosition) (*pb.Position, error) {
	base := raw.NetPositionBase
	listing, ok := c.listingFor(base.Uic, base.AssetType)
	if !ok || listing.ISIN == "" {
		return nil, fmt.Errorf("position uic %d: %w", base.Uic, supplier.ErrUnmappedPosition)
	}
	qty, err := wholeCount(base.Amount)
	if err != nil {
		return nil, fmt.Errorf("position uic %d: %w", base.Uic, supplier.ErrUnavailable)
	}
	pos := &pb.Position{
		Symbol: listing.Symbol,
		Isin:   listing.ISIN,
		Amount: qty,
	}
	if raw.NetPositionView.MarketValueOpen != nil {
		if !moneyOK(*raw.NetPositionView.MarketValueOpen) {
			return nil, fmt.Errorf("position uic %d cost: %w", base.Uic, supplier.ErrUnavailable)
		}
		pos.CostBasis = *raw.NetPositionView.MarketValueOpen
	}
	return pos, nil
}

func mapSaxoOrder(raw saxoOpenOrder, listing Listing) (*pb.Order, error) {
	if strings.TrimSpace(string(raw.OrderID)) == "" {
		return nil, fmt.Errorf("saxo order missing id: %w", supplier.ErrUnavailable)
	}
	var sign int32
	switch strings.ToLower(strings.TrimSpace(raw.BuySell)) {
	case "buy":
		sign = 1
	case "sell":
		sign = -1
	default:
		return nil, fmt.Errorf("saxo side %q: %w", raw.BuySell, supplier.ErrUnavailable)
	}
	qty, err := wholeCount(raw.Amount)
	if err != nil || qty < 0 {
		return nil, fmt.Errorf("saxo amount: %w", supplier.ErrUnavailable)
	}
	filled, err := wholeCount(raw.FilledAmount)
	if err != nil || filled < 0 {
		return nil, fmt.Errorf("saxo filled amount: %w", supplier.ErrUnavailable)
	}
	orderType, err := saxoOrderType(raw.OpenOrderType)
	if err != nil {
		return nil, err
	}
	out := &pb.Order{
		Symbol:        listing.Symbol,
		Isin:          listing.ISIN,
		Amount:        sign * qty,
		Type:          orderType,
		TimeInForce:   saxoTIF(raw.Duration.DurationType),
		Status:        fillStatus(filled, qty),
		FilledAmount:  sign * filled,
		BrokerOrderId: string(raw.OrderID),
	}
	if orderType == pb.OrderType_ORDER_TYPE_LIMIT && raw.Price != nil {
		if !moneyOK(*raw.Price) {
			return nil, fmt.Errorf("saxo price: %w", supplier.ErrUnavailable)
		}
		out.LimitPrice = *raw.Price
	}
	return out, nil
}

func saxoOrderType(raw string) (pb.OrderType, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "market":
		return pb.OrderType_ORDER_TYPE_MARKET, nil
	case "limit":
		return pb.OrderType_ORDER_TYPE_LIMIT, nil
	default:
		return pb.OrderType_ORDER_TYPE_UNSPECIFIED, fmt.Errorf("saxo type %q: %w", raw, supplier.ErrUnavailable)
	}
}

func saxoTIF(raw string) pb.TimeInForce {
	if strings.EqualFold(strings.TrimSpace(raw), "DayOrder") {
		return pb.TimeInForce_TIME_IN_FORCE_DAY
	}
	return pb.TimeInForce_TIME_IN_FORCE_UNSPECIFIED
}

func (c *SaxoClient) get(ctx context.Context, path string) ([]byte, error) {
	return c.exchange(ctx, http.MethodGet, path, nil)
}

func (c *SaxoClient) post(ctx context.Context, path string, payload []byte) ([]byte, error) {
	return c.exchange(ctx, http.MethodPost, path, payload)
}

func (c *SaxoClient) exchange(ctx context.Context, method, path string, payload []byte) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var body io.Reader
	if payload != nil {
		body = bytes.NewReader(payload)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return nil, fmt.Errorf("saxo request: %w", err)
	}
	req.Header.Set("Authorization", saxoAuthorization(c.token))
	req.Header.Set("Accept", "application/json")
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("saxo %s: %w", method, supplier.ErrUnavailable)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("saxo read: %w", supplier.ErrUnavailable)
	}
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, saxoHTTPError(resp.StatusCode, raw)
	}
	return raw, nil
}

func saxoAuthorization(token string) string {
	token = strings.TrimSpace(token)
	if token == "" {
		return "Bearer"
	}
	return "Bearer " + token
}

func saxoHTTPError(code int, body []byte) error {
	msg := saxoMessage(body)
	switch {
	case code == http.StatusUnauthorized || code >= http.StatusInternalServerError:
		if msg == "" {
			return fmt.Errorf("saxo http %d: %w", code, supplier.ErrUnavailable)
		}
		return fmt.Errorf("saxo http %d: %s: %w", code, msg, supplier.ErrUnavailable)
	case code >= http.StatusBadRequest:
		if msg == "" {
			return fmt.Errorf("saxo http %d: %w", code, supplier.ErrRejected)
		}
		return fmt.Errorf("saxo http %d: %s: %w", code, msg, supplier.ErrRejected)
	default:
		return fmt.Errorf("saxo http %d: %w", code, supplier.ErrUnavailable)
	}
}

func saxoMessage(body []byte) string {
	var payload struct {
		Message   string        `json:"Message"`
		ErrorInfo saxoErrorInfo `json:"ErrorInfo"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return ""
	}
	msg := strings.TrimSpace(payload.Message)
	if msg == "" {
		msg = strings.TrimSpace(payload.ErrorInfo.Message)
	}
	if len(msg) > 512 {
		return msg[:512]
	}
	return msg
}

func moneyOK(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}

func wholeCount(value float64) (int32, error) {
	if !moneyOK(value) || value != math.Trunc(value) {
		return 0, fmt.Errorf("quantity %v", value)
	}
	if value > math.MaxInt32 || value < math.MinInt32 {
		return 0, fmt.Errorf("quantity %v", value)
	}
	return int32(value), nil
}

type saxoAccountPage struct {
	Data []saxoAccount `json:"Data"`
	Next string        `json:"__next"`
}

type saxoAccount struct {
	AccountKey string `json:"AccountKey"`
	Currency   string `json:"Currency"`
}

type saxoBalance struct {
	CashBalance float64 `json:"CashBalance"`
	Currency    string  `json:"Currency"`
	TotalValue  float64 `json:"TotalValue"`
}

type saxoNetPositionPage struct {
	Data []saxoNetPosition `json:"Data"`
	Next string            `json:"__next"`
}

type saxoNetPosition struct {
	NetPositionBase saxoNetPositionBase `json:"NetPositionBase"`
	NetPositionView saxoNetPositionView `json:"NetPositionView"`
}

type saxoNetPositionBase struct {
	Amount    float64 `json:"Amount"`
	AssetType string  `json:"AssetType"`
	Uic       int64   `json:"Uic"`
}

type saxoNetPositionView struct {
	MarketValueOpen *float64 `json:"MarketValueOpen"`
}

type saxoOrderPage struct {
	Data []saxoOpenOrder `json:"Data"`
	Next string          `json:"__next"`
}

type saxoOpenOrder struct {
	Amount        float64           `json:"Amount"`
	AssetType     string            `json:"AssetType"`
	BuySell       string            `json:"BuySell"`
	Duration      saxoOrderDuration `json:"Duration"`
	FilledAmount  float64           `json:"FilledAmount"`
	OpenOrderType string            `json:"OpenOrderType"`
	OrderID       saxoID            `json:"OrderId"`
	Price         *float64          `json:"Price"`
	Status        string            `json:"Status"`
	Uic           int64             `json:"Uic"`
}

type saxoPlaceResponse struct {
	OrderID   saxoID        `json:"OrderId"`
	ErrorInfo saxoErrorInfo `json:"ErrorInfo"`
	ErrorCode string        `json:"ErrorCode"`
	Message   string        `json:"Message"`
}

type saxoErrorInfo struct {
	ErrorCode string `json:"ErrorCode"`
	Message   string `json:"Message"`
}

func (r saxoPlaceResponse) errorMessage() string {
	if msg := strings.TrimSpace(r.ErrorInfo.Message); msg != "" {
		return msg
	}
	if strings.TrimSpace(r.ErrorInfo.ErrorCode) != "" {
		return strings.TrimSpace(r.ErrorInfo.ErrorCode)
	}
	if msg := strings.TrimSpace(r.Message); msg != "" {
		return msg
	}
	return strings.TrimSpace(r.ErrorCode)
}

// saxoID accepts the string OrderId in the reference schema and the numeric
// OrderId in the order-placement examples.
type saxoID string

func (s *saxoID) UnmarshalJSON(raw []byte) error {
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
		*s = saxoID(strings.TrimSpace(value))
		return nil
	}
	var number json.Number
	if err := json.Unmarshal(raw, &number); err != nil {
		return err
	}
	*s = saxoID(number.String())
	return nil
}
