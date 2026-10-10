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
	// Demo REST origin. Live host api.ig.com is refused unless AllowLiveURL
	// is set. That flag does not unlock mode LIVE.
	// https://labs.ig.com/rest-trading-api-guide
	igDemoBaseURL = "https://demo-api.ig.com/gateway/deal"
	igLiveHost    = "api.ig.com"

	// One session. CST and X-SECURITY-TOKEN are reused. A later 401 does not call this again.
	// https://labs.ig.com/reference/session
	igSessionPath = "/session"
	// Account currency, balance, and profitLoss.
	// https://labs.ig.com/reference/accounts
	igAccountsPath = "/accounts"
	// Open positions. market.epic is the listing id. position.size is deal size.
	// https://labs.ig.com/reference/positions
	igPositionsPath = "/positions"
	// Create an OTC position. v2. Market omits level. Limit sets level.
	// https://labs.ig.com/reference/positions-otc
	igCreatePositionPath = "/positions/otc"
	// Open working orders. The JSON array is workingOrders.
	// https://labs.ig.com/reference/working-orders
	igWorkingOrdersPath = "/working-orders"

	// Non-expiring expiry. The create constraint allows "-", "DFB", or a dated code.
	// https://labs.ig.com/reference/positions-otc
	igExpiry = "-"

	igVersionSession  = "2"
	igVersionAccounts = "1"
	igVersionDeal     = "2"

	defaultIGCurrency = "USD"
)

// IGConfig is supplied by module.go. NewIG does not read the environment.
type IGConfig struct {
	// BaseURL is the gateway origin including /gateway/deal. Empty selects the demo origin.
	BaseURL string
	// APIKey is sent as X-IG-API-KEY. Empty stays empty.
	APIKey string
	// Identifier is the session login identifier. Empty stays empty.
	Identifier string
	// Password is the session login password. Empty stays empty.
	Password string
	// Currency is compared to the account currency. Empty means USD.
	Currency string
	// AllowLiveURL permits host api.ig.com. It does not unlock mode LIVE.
	AllowLiveURL bool
}

// IGClient is the IG REST trading HTTP adapter.
type IGClient struct {
	baseURL       string
	apiKey        string
	currency      string
	accountID     string
	cst           string
	securityToken string
	httpClient    *http.Client
	byISIN        map[string]Listing
	byEpic        map[string]Listing
}

var _ supplier.Trading = (*IGClient)(nil)

// NewIG builds a trading client and posts one session. book and client are required.
// A nil book or client is an error. The demo origin is the default base URL.
// Login is not repeated. A later HTTP 401 is ErrUnavailable.
func NewIG(cfg IGConfig, book *Book, client *http.Client) (*IGClient, error) {
	if book == nil {
		return nil, errors.New("ig trading: nil listings")
	}
	if client == nil {
		return nil, errors.New("ig trading: nil http client")
	}
	base, err := igBaseURL(cfg)
	if err != nil {
		return nil, err
	}
	currency := strings.ToUpper(strings.TrimSpace(cfg.Currency))
	if currency == "" {
		currency = defaultIGCurrency
	}

	byISIN := make(map[string]Listing)
	byEpic := make(map[string]Listing)
	ambiguous := make(map[string]struct{})
	for _, row := range book.List() {
		byISIN[row.ISIN] = row
		if row.IGEpic == "" || row.IGShareSize <= 0 {
			continue
		}
		if _, dup := ambiguous[row.IGEpic]; dup {
			continue
		}
		if _, seen := byEpic[row.IGEpic]; seen {
			delete(byEpic, row.IGEpic)
			ambiguous[row.IGEpic] = struct{}{}
			continue
		}
		byEpic[row.IGEpic] = row
	}

	ig := &IGClient{
		baseURL:    base,
		apiKey:     cfg.APIKey,
		currency:   currency,
		httpClient: client,
		byISIN:     byISIN,
		byEpic:     byEpic,
	}
	if err := ig.login(context.Background(), cfg.Identifier, cfg.Password); err != nil {
		return nil, err
	}
	return ig, nil
}

func igBaseURL(cfg IGConfig) (string, error) {
	raw := strings.TrimSpace(cfg.BaseURL)
	if raw == "" {
		raw = igDemoBaseURL
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("ig trading: base url %q", cfg.BaseURL)
	}
	if parsed.Scheme != "https" && parsed.Scheme != "http" {
		return "", fmt.Errorf("ig trading: base url %q", cfg.BaseURL)
	}
	if parsed.User != nil {
		return "", errors.New("ig trading: base url has userinfo")
	}
	if strings.EqualFold(parsed.Hostname(), igLiveHost) && !cfg.AllowLiveURL {
		return "", fmt.Errorf("ig trading: live host %s refused", parsed.Hostname())
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

// GetPortfolio reads the account currency and balance, then open positions.
func (c *IGClient) GetPortfolio(ctx context.Context) (*pb.Portfolio, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	accountBody, err := c.get(ctx, igAccountsPath, igVersionAccounts)
	if err != nil {
		return nil, err
	}
	var accounts igAccountPage
	if err := json.Unmarshal(accountBody, &accounts); err != nil {
		return nil, fmt.Errorf("ig accounts: %w", supplier.ErrUnavailable)
	}
	account, err := c.selectAccount(accounts.Accounts)
	if err != nil {
		return nil, err
	}
	currency := strings.ToUpper(strings.TrimSpace(account.Currency))
	if currency == "" {
		return nil, fmt.Errorf("ig account currency: %w", supplier.ErrUnavailable)
	}
	if currency != c.currency {
		return nil, fmt.Errorf("account currency %q: %w", account.Currency, supplier.ErrCurrency)
	}
	if account.Balance == nil || account.Balance.Balance == nil || account.Balance.ProfitLoss == nil {
		return nil, fmt.Errorf("ig balance: %w", supplier.ErrUnavailable)
	}
	cash := *account.Balance.Balance
	profit := *account.Balance.ProfitLoss
	equity := cash + profit
	if !moneyOK(cash) || !moneyOK(profit) || !moneyOK(equity) {
		return nil, fmt.Errorf("ig balance: %w", supplier.ErrUnavailable)
	}

	positionBody, err := c.get(ctx, igPositionsPath, igVersionDeal)
	if err != nil {
		return nil, err
	}
	var positions igPositionPage
	if err := json.Unmarshal(positionBody, &positions); err != nil {
		return nil, fmt.Errorf("ig positions: %w", supplier.ErrUnavailable)
	}
	mapped := make([]*pb.Position, 0, len(positions.Positions))
	for _, raw := range positions.Positions {
		pos, err := c.mapPosition(raw)
		if err != nil {
			return nil, err
		}
		mapped = append(mapped, pos)
	}

	return &pb.Portfolio{
		Timestamp:      timestamppb.Now(),
		Cash:           cash,
		PortfolioValue: equity,
		Positions:      mapped,
	}, nil
}

// PlaceOrder posts one OTC position. Deal size is abs(amount) / ig_share_size
// and must be a positive integer. A non-divisible delta does not POST.
// The first error is returned as-is.
func (c *IGClient) PlaceOrder(ctx context.Context, order *pb.Order) (*pb.Order, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	listing, req, err := c.orderRequest(order)
	if err != nil {
		return nil, err
	}
	payload, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("ig order: %w", err)
	}
	body, err := c.post(ctx, igCreatePositionPath, igVersionDeal, payload)
	if err != nil {
		return nil, err
	}
	var placed igCreateResponse
	if err := json.Unmarshal(body, &placed); err != nil {
		return nil, fmt.Errorf("ig order: %w", supplier.ErrUnavailable)
	}
	id := strings.TrimSpace(placed.DealReference)
	if id == "" {
		return nil, fmt.Errorf("ig order missing id: %w", supplier.ErrUnavailable)
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

// GetOrders reads open working orders. An empty slice is returned only when
// workingOrders is empty. An open working order has no fill size.
func (c *IGClient) GetOrders(ctx context.Context) ([]*pb.Order, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	body, err := c.get(ctx, igWorkingOrdersPath, igVersionDeal)
	if err != nil {
		return nil, err
	}
	var page igWorkingOrderPage
	if err := json.Unmarshal(body, &page); err != nil {
		return nil, fmt.Errorf("ig orders: %w", supplier.ErrUnavailable)
	}
	out := make([]*pb.Order, 0, len(page.WorkingOrders))
	for _, raw := range page.WorkingOrders {
		mapped, err := c.mapWorkingOrder(raw)
		if err != nil {
			return nil, err
		}
		out = append(out, mapped)
	}
	return out, nil
}

func (c *IGClient) selectAccount(accounts []igAccount) (igAccount, error) {
	if len(accounts) == 0 {
		return igAccount{}, fmt.Errorf("ig accounts empty: %w", supplier.ErrUnavailable)
	}
	if c.accountID != "" {
		for _, account := range accounts {
			if strings.TrimSpace(account.AccountID) == c.accountID {
				return account, nil
			}
		}
		return igAccount{}, fmt.Errorf("ig account: %w", supplier.ErrUnavailable)
	}
	var preferred []igAccount
	for _, account := range accounts {
		if account.Preferred {
			preferred = append(preferred, account)
		}
	}
	if len(preferred) == 1 {
		return preferred[0], nil
	}
	if len(accounts) == 1 {
		return accounts[0], nil
	}
	return igAccount{}, fmt.Errorf("ig account: %w", supplier.ErrUnavailable)
}

// igCreatePosition is POST /positions/otc v2.
// Market orders omit level. Limit orders set level to the limit price.
// forceOpen is false: a share delta may reduce an existing position.
// guaranteedStop is false: stops are out of scope. Both are NotNull.
// Position timeInForce is a fill strategy, not a day duration, so it is omitted.
// https://labs.ig.com/reference/positions-otc
type igCreatePosition struct {
	CurrencyCode   string   `json:"currencyCode"`
	Direction      string   `json:"direction"`
	Epic           string   `json:"epic"`
	Expiry         string   `json:"expiry"`
	ForceOpen      bool     `json:"forceOpen"`
	GuaranteedStop bool     `json:"guaranteedStop"`
	Level          *float64 `json:"level,omitempty"`
	OrderType      string   `json:"orderType"`
	Size           int64    `json:"size"`
}

func (c *IGClient) orderRequest(order *pb.Order) (Listing, igCreatePosition, error) {
	if order == nil {
		return Listing{}, igCreatePosition{}, fmt.Errorf("nil order: %w", supplier.ErrRejected)
	}
	isin := normalizeISIN(order.GetIsin())
	if isin == "" {
		return Listing{}, igCreatePosition{}, fmt.Errorf("empty isin: %w", supplier.ErrNotTradable)
	}
	listing, ok := c.byISIN[isin]
	if !ok || listing.IGEpic == "" || listing.IGShareSize <= 0 {
		return Listing{}, igCreatePosition{}, fmt.Errorf("isin %s: %w", isin, supplier.ErrNotTradable)
	}
	size, direction, err := igDealSize(order.GetAmount(), listing.IGShareSize)
	if err != nil {
		return Listing{}, igCreatePosition{}, err
	}

	var orderType string
	switch order.GetType() {
	case pb.OrderType_ORDER_TYPE_MARKET:
		orderType = "MARKET"
	case pb.OrderType_ORDER_TYPE_LIMIT:
		orderType = "LIMIT"
	case pb.OrderType_ORDER_TYPE_UNSPECIFIED:
		return Listing{}, igCreatePosition{}, fmt.Errorf("order type %s: %w", order.GetType(), supplier.ErrRejected)
	default:
		return Listing{}, igCreatePosition{}, fmt.Errorf("order type %s: %w", order.GetType(), supplier.ErrRejected)
	}
	switch order.GetTimeInForce() {
	case pb.TimeInForce_TIME_IN_FORCE_UNSPECIFIED, pb.TimeInForce_TIME_IN_FORCE_DAY:
	default:
		return Listing{}, igCreatePosition{}, fmt.Errorf("time in force %s: %w", order.GetTimeInForce(), supplier.ErrRejected)
	}

	req := igCreatePosition{
		CurrencyCode:   listing.Currency,
		Direction:      direction,
		Epic:           listing.IGEpic,
		Expiry:         igExpiry,
		ForceOpen:      false,
		GuaranteedStop: false,
		OrderType:      orderType,
		Size:           size,
	}
	if orderType == "LIMIT" {
		price := order.GetLimitPrice()
		if price <= 0 || math.IsNaN(price) || math.IsInf(price, 0) {
			return Listing{}, igCreatePosition{}, fmt.Errorf("limit price: %w", supplier.ErrRejected)
		}
		req.Level = &price
	}
	return listing, req, nil
}

// igDealSize is abs(amount) / ig_share_size. It must be a positive integer.
// A non-divisible delta is ErrUnsized and the caller does not POST.
func igDealSize(amount int32, shareSize int64) (int64, string, error) {
	if amount == 0 || amount == math.MinInt32 {
		return 0, "", fmt.Errorf("amount: %w", supplier.ErrRejected)
	}
	if shareSize <= 0 {
		return 0, "", fmt.Errorf("share size: %w", supplier.ErrNotTradable)
	}
	abs := int64(amount)
	direction := "BUY"
	if amount < 0 {
		direction = "SELL"
		abs = -abs
	}
	if abs%shareSize != 0 {
		return 0, "", fmt.Errorf("deal size: %w", supplier.ErrUnsized)
	}
	size := abs / shareSize
	if size <= 0 {
		return 0, "", fmt.Errorf("deal size: %w", supplier.ErrUnsized)
	}
	return size, direction, nil
}

func (c *IGClient) mapPosition(raw igOpenPosition) (*pb.Position, error) {
	epic := strings.TrimSpace(raw.Market.Epic)
	listing, ok := c.byEpic[epic]
	if !ok || listing.ISIN == "" {
		return nil, fmt.Errorf("position epic %s: %w", epic, supplier.ErrUnmappedPosition)
	}
	if raw.Position.Size < 0 || !moneyOK(raw.Position.Size) {
		return nil, fmt.Errorf("position epic %s: %w", epic, supplier.ErrUnavailable)
	}
	shares, err := igShares(raw.Position.Size, listing.IGShareSize)
	if err != nil || shares < 0 {
		return nil, fmt.Errorf("position epic %s: %w", epic, supplier.ErrUnavailable)
	}
	sign, err := igSideSign(raw.Position.Direction)
	if err != nil {
		return nil, err
	}
	pos := &pb.Position{
		Symbol: listing.Symbol,
		Isin:   listing.ISIN,
		Amount: sign * shares,
	}
	if raw.Position.Level != nil {
		if !moneyOK(*raw.Position.Level) {
			return nil, fmt.Errorf("position epic %s cost: %w", epic, supplier.ErrUnavailable)
		}
		pos.CostBasis = *raw.Position.Level
	}
	return pos, nil
}

func (c *IGClient) mapWorkingOrder(raw igWorkingOrder) (*pb.Order, error) {
	data := raw.WorkingOrderData
	epic := strings.TrimSpace(data.Epic)
	if epic == "" {
		epic = strings.TrimSpace(raw.MarketData.Epic)
	}
	listing, ok := c.byEpic[epic]
	if !ok || listing.ISIN == "" {
		return nil, fmt.Errorf("order epic %s: %w", epic, supplier.ErrNotTradable)
	}
	if strings.TrimSpace(data.DealID) == "" {
		return nil, fmt.Errorf("ig order missing id: %w", supplier.ErrUnavailable)
	}
	if data.OrderSize < 0 || !moneyOK(data.OrderSize) {
		return nil, fmt.Errorf("ig size: %w", supplier.ErrUnavailable)
	}
	shares, err := igShares(data.OrderSize, listing.IGShareSize)
	if err != nil || shares <= 0 {
		return nil, fmt.Errorf("ig size: %w", supplier.ErrUnavailable)
	}
	sign, err := igSideSign(data.Direction)
	if err != nil {
		return nil, err
	}
	orderType, err := igOrderType(data.OrderType)
	if err != nil {
		return nil, err
	}
	out := &pb.Order{
		Symbol:        listing.Symbol,
		Isin:          listing.ISIN,
		Amount:        sign * shares,
		Type:          orderType,
		TimeInForce:   igTIF(data.TimeInForce),
		Status:        pb.OrderStatus_ORDER_STATUS_ACCEPTED,
		BrokerOrderId: strings.TrimSpace(data.DealID),
	}
	if orderType == pb.OrderType_ORDER_TYPE_LIMIT {
		if data.OrderLevel == nil || !moneyOK(*data.OrderLevel) || *data.OrderLevel <= 0 {
			return nil, fmt.Errorf("ig level: %w", supplier.ErrUnavailable)
		}
		out.LimitPrice = *data.OrderLevel
	}
	return out, nil
}

func igShares(dealSize float64, shareSize int64) (int32, error) {
	if shareSize <= 0 || !moneyOK(dealSize) {
		return 0, fmt.Errorf("deal size %v", dealSize)
	}
	return wholeCount(dealSize * float64(shareSize))
}

func igSideSign(direction string) (int32, error) {
	switch strings.ToUpper(strings.TrimSpace(direction)) {
	case "BUY":
		return 1, nil
	case "SELL":
		return -1, nil
	default:
		return 0, fmt.Errorf("ig direction %q: %w", direction, supplier.ErrUnavailable)
	}
}

func igOrderType(raw string) (pb.OrderType, error) {
	switch strings.ToUpper(strings.TrimSpace(raw)) {
	case "MARKET":
		return pb.OrderType_ORDER_TYPE_MARKET, nil
	case "LIMIT":
		return pb.OrderType_ORDER_TYPE_LIMIT, nil
	default:
		return pb.OrderType_ORDER_TYPE_UNSPECIFIED, fmt.Errorf("ig type %q: %w", raw, supplier.ErrUnavailable)
	}
}

// GOOD_TILL_DATE is the dated working order and is the platform day order.
// GOOD_TILL_CANCELLED is not a day order.
// https://labs.ig.com/reference/working-orders
func igTIF(raw string) pb.TimeInForce {
	if strings.EqualFold(strings.TrimSpace(raw), "GOOD_TILL_DATE") {
		return pb.TimeInForce_TIME_IN_FORCE_DAY
	}
	return pb.TimeInForce_TIME_IN_FORCE_UNSPECIFIED
}

// igSessionRequest is POST /session v2. encryptedPassword is false: the password is not encrypted.
// CST and X-SECURITY-TOKEN come back as response headers, not as another login.
// https://labs.ig.com/reference/session
// https://labs.ig.com/rest-trading-api-guide
type igSessionRequest struct {
	Identifier        string `json:"identifier"`
	Password          string `json:"password"`
	EncryptedPassword bool   `json:"encryptedPassword"`
}

type igSessionResponse struct {
	CurrentAccountID string `json:"currentAccountId"`
}

func (c *IGClient) login(ctx context.Context, identifier, password string) error {
	payload, err := json.Marshal(igSessionRequest{
		Identifier:        identifier,
		Password:          password,
		EncryptedPassword: false,
	})
	if err != nil {
		return fmt.Errorf("ig session: %w", err)
	}
	// identifier and password are not stored. This POST is the only login.
	body, header, err := c.exchange(ctx, http.MethodPost, igSessionPath, igVersionSession, payload, false)
	if err != nil {
		if errors.Is(err, supplier.ErrRejected) {
			return fmt.Errorf("ig session: %w", supplier.ErrUnavailable)
		}
		return err
	}
	c.cst = header.Get("CST")
	c.securityToken = header.Get("X-SECURITY-TOKEN")
	var parsed igSessionResponse
	if len(bytes.TrimSpace(body)) > 0 {
		if err := json.Unmarshal(body, &parsed); err != nil {
			return fmt.Errorf("ig session: %w", supplier.ErrUnavailable)
		}
	}
	c.accountID = strings.TrimSpace(parsed.CurrentAccountID)
	return nil
}

func (c *IGClient) get(ctx context.Context, rel, version string) ([]byte, error) {
	body, _, err := c.exchange(ctx, http.MethodGet, rel, version, nil, true)
	return body, err
}

func (c *IGClient) post(ctx context.Context, rel, version string, payload []byte) ([]byte, error) {
	body, _, err := c.exchange(ctx, http.MethodPost, rel, version, payload, true)
	return body, err
}

func (c *IGClient) exchange(ctx context.Context, method, rel, version string, payload []byte, authed bool) ([]byte, http.Header, error) {
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	var body io.Reader
	if payload != nil {
		body = bytes.NewReader(payload)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+rel, body)
	if err != nil {
		return nil, nil, fmt.Errorf("ig request: %w", err)
	}
	req.Header.Set("X-IG-API-KEY", c.apiKey)
	req.Header.Set("Accept", "application/json; charset=UTF-8")
	req.Header.Set("Version", version)
	if authed {
		req.Header.Set("CST", c.cst)
		req.Header.Set("X-SECURITY-TOKEN", c.securityToken)
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, nil, ctx.Err()
		}
		return nil, nil, fmt.Errorf("ig %s: %w", method, supplier.ErrUnavailable)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		if ctx.Err() != nil {
			return nil, nil, ctx.Err()
		}
		return nil, nil, fmt.Errorf("ig read: %w", supplier.ErrUnavailable)
	}
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, nil, igHTTPError(resp.StatusCode, raw)
	}
	return raw, resp.Header.Clone(), nil
}

func igHTTPError(code int, body []byte) error {
	msg := igMessage(body)
	switch {
	case code == http.StatusUnauthorized || code >= http.StatusInternalServerError:
		if msg == "" {
			return fmt.Errorf("ig http %d: %w", code, supplier.ErrUnavailable)
		}
		return fmt.Errorf("ig http %d: %s: %w", code, msg, supplier.ErrUnavailable)
	case code >= http.StatusBadRequest:
		if msg == "" {
			return fmt.Errorf("ig http %d: %w", code, supplier.ErrRejected)
		}
		return fmt.Errorf("ig http %d: %s: %w", code, msg, supplier.ErrRejected)
	default:
		return fmt.Errorf("ig http %d: %w", code, supplier.ErrUnavailable)
	}
}

func igMessage(body []byte) string {
	var payload struct {
		ErrorCode string `json:"errorCode"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return ""
	}
	msg := strings.TrimSpace(payload.ErrorCode)
	if len(msg) > 512 {
		return msg[:512]
	}
	return msg
}

type igAccountPage struct {
	Accounts []igAccount `json:"accounts"`
}

type igAccount struct {
	AccountID string     `json:"accountId"`
	Currency  string     `json:"currency"`
	Preferred bool       `json:"preferred"`
	Balance   *igBalance `json:"balance"`
}

type igBalance struct {
	Available  *float64 `json:"available"`
	Balance    *float64 `json:"balance"`
	Deposit    *float64 `json:"deposit"`
	ProfitLoss *float64 `json:"profitLoss"`
}

type igPositionPage struct {
	Positions []igOpenPosition `json:"positions"`
}

type igOpenPosition struct {
	Market   igMarket       `json:"market"`
	Position igPositionData `json:"position"`
}

type igMarket struct {
	Epic   string `json:"epic"`
	Expiry string `json:"expiry"`
}

type igPositionData struct {
	DealID    string   `json:"dealId"`
	Direction string   `json:"direction"`
	Level     *float64 `json:"level"`
	Size      float64  `json:"size"`
}

type igCreateResponse struct {
	DealReference string `json:"dealReference"`
}

type igWorkingOrderPage struct {
	WorkingOrders []igWorkingOrder `json:"workingOrders"`
}

type igWorkingOrder struct {
	MarketData       igMarket           `json:"marketData"`
	WorkingOrderData igWorkingOrderData `json:"workingOrderData"`
}

type igWorkingOrderData struct {
	DealID      string   `json:"dealId"`
	Direction   string   `json:"direction"`
	Epic        string   `json:"epic"`
	OrderLevel  *float64 `json:"orderLevel"`
	OrderSize   float64  `json:"orderSize"`
	OrderType   string   `json:"orderType"`
	TimeInForce string   `json:"timeInForce"`
}
