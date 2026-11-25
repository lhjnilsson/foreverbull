package marketdata

import (
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/internal/usercopy"
	"github.com/lhjnilsson/foreverbull/pkg/finance/internal/mapping"
	"github.com/lhjnilsson/foreverbull/pkg/finance/supplier"
	pb "github.com/lhjnilsson/foreverbull/pkg/pb/finance"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	defaultStooqBaseURL = "https://stooq.com"
	stooqDailyPath      = "/q/d/l/"
	stooqUserAgent      = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"
)

// ErrStooqMetadata is returned by GetAsset/SearchAsset/GetIndex. Stooq is GetOHLC-only.
var ErrStooqMetadata = errors.New("stooq does not implement GetAsset/SearchAsset/GetIndex")

// defaultStooqUS is a small Yahoo-style → Stooq daily-code table for US equities/ETFs.
// Canonical form (AAPL) is what Postgres/ohlc.symbol stores. A symbol that is
// not in this table and has no exchange suffix uses the configured exchange.
var defaultStooqUS = map[string]string{
	"AAPL":  "aapl.us",
	"SPY":   "spy.us",
	"MSFT":  "msft.us",
	"GOOGL": "googl.us",
	"NVDA":  "nvda.us",
}

// stooqSupportedSuffixes lists the supported suffixes. Only .US is accepted.
func stooqSupportedSuffixes() string {
	return mapping.SupportedStooqSuffixes()
}

// stooqRefusal is the user-facing text for a symbol Stooq can't be asked
// for. It matches ErrStooqExchange with errors.Is.
type stooqRefusal struct{ msg string }

func (e *stooqRefusal) Error() string { return e.msg }
func (e *stooqRefusal) Unwrap() error { return ErrStooqExchange }

// errStooqInvalidTicker never echoes the symbol: it failed the bound check.
var errStooqInvalidTicker = &stooqRefusal{
	msg: "stooq can't use this symbol because it isn't a valid ticker. Check the spelling, for example AAPL.",
}

// ErrStooqExchange is returned by GetOHLC for a symbol whose exchange suffix
// has no known Stooq code. The sentence a person reads is a stooqRefusal,
// which unwraps to this sentinel.
var ErrStooqExchange = errors.New("stooq has no code for this exchange")

// stooqExchangeStep is the end of the GetOHLC sentence. Create replaces it
// with its own next step.
const stooqExchangeStep = "Try another provider with --provider."

// stooqExchangeLead is the refusal without that step. symbol is the asked
// ticker, suffix included, already in the form the sentence prints.
func stooqExchangeLead(symbol, suffix string) string {
	return fmt.Sprintf(
		"stooq doesn't support the .%s exchange (%s). Supported suffixes: %s.",
		strings.ToUpper(suffix), strings.ToUpper(symbol), stooqSupportedSuffixes())
}

// stooqExchangeRefusal is the sentence a person sees for a suffix Stooq has
// no code for. Ingest shows it as written.
func stooqExchangeRefusal(symbol, suffix string) string {
	return stooqExchangeLead(symbol, suffix) + " " + stooqExchangeStep
}

// StooqExchangeLead is that refusal for symbol, without the provider step.
// ok is false when symbol has no exchange suffix. symbol may be any case.
func StooqExchangeLead(symbol string) (string, bool) {
	upper, suffix, ok := stooqExchangeParts(symbol)
	if !ok {
		return "", false
	}
	return stooqExchangeLead(upper, suffix), true
}

// IsStooqExchangeRefusal reports whether text is the full GetOHLC sentence
// for symbol, step included. symbol may be any case. Create matches this,
// then uses StooqExchangeLead for its own message.
func IsStooqExchangeRefusal(text, symbol string) bool {
	upper, suffix, ok := stooqExchangeParts(symbol)
	if !ok {
		return false
	}
	return text == stooqExchangeRefusal(upper, suffix)
}

func stooqExchangeParts(symbol string) (upper, suffix string, ok bool) {
	upper = strings.ToUpper(strings.TrimSpace(symbol))
	dot := strings.LastIndexByte(upper, '.')
	if dot <= 0 || dot == len(upper)-1 {
		return "", "", false
	}
	return upper, upper[dot+1:], true
}

var _ supplier.Marketdata = (*StooqClient)(nil)

// StooqClient fetches daily OHLC from Stooq CSV. Metadata methods do not scrape Stooq.
type StooqClient struct {
	baseURL    string
	httpClient *http.Client
	// limiter spaces the Stooq requests. NewStooqClient sets the
	// process-wide Stooq limiter, so every client shares one rate.
	limiter     *requestLimiter
	toStooq     map[string]string // canonical upper → stooq code
	toCanonical map[string]string // stooq code lower → canonical upper
	// exchange is Stooq's suffix. Only us is accepted. Empty is refused.
	exchange string
	// now replaces the clock, so tests can fix today's session.
	now func() time.Time
	// classes is shared with Yahoo when the broker built this client.
	classes *mapping.Classes
}

// StooqOption configures a StooqClient.
type StooqOption func(*StooqClient) error

// WithStooqExchange sets the market used when a symbol does not already name
// one. Only US is accepted. An empty or other value fails the client before
// any request.
func WithStooqExchange(exchange string) StooqOption {
	return func(c *StooqClient) error {
		suffix, err := NormalizeStooqExchange(exchange)
		if err != nil {
			return err
		}
		c.exchange = suffix
		return nil
	}
}

// userSentence is a sentence a person reads. errors.New rejects a final
// period; the text is that sentence, unchanged.
type userSentence struct{ text string }

func (e *userSentence) Error() string { return e.text }

// NormalizeStooqExchange returns Stooq's suffix for an exchange name.
// Only US is accepted. An empty value and any other value are the sentences
// a person sees.
func NormalizeStooqExchange(exchange string) (string, error) {
	switch strings.ToUpper(strings.TrimSpace(exchange)) {
	case "":
		return "", &userSentence{text: usercopy.StooqExchangeMissing}
	case "US":
		return "us", nil
	default:
		return "", errors.New(usercopy.StooqExchangeUnsupported(exchange))
	}
}

// ConfiguredStooqExchange reads STOOQ_EXCHANGE. Unset and unsupported values
// fail with the sentence a person sees.
func ConfiguredStooqExchange() (string, error) {
	return NormalizeStooqExchange(environment.GetStooqExchange())
}

// WithStooqBaseURL overrides https://stooq.com (httptest in unit tests).
func WithStooqBaseURL(baseURL string) StooqOption {
	return func(c *StooqClient) error {
		c.baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
		return nil
	}
}

// WithStooqHTTPClient replaces the default client (30s timeout).
func WithStooqHTTPClient(client *http.Client) StooqOption {
	return func(c *StooqClient) error {
		if client == nil {
			return errors.New("stooq: nil HTTP client")
		}
		c.httpClient = client
		return nil
	}
}

// WithStooqSymbolMap merges extra canonical→Stooq codes (overrides the built-in table).
func WithStooqSymbolMap(m map[string]string) StooqOption {
	return func(c *StooqClient) error {
		for canonical, code := range m {
			if err := c.addMapping(canonical, code); err != nil {
				return err
			}
		}
		return nil
	}
}

// WithStooqSymbolMapFile loads extra canonical,stooq CSV rows. A header of
// canonical,stooq is optional. Extra rows override the built-in table.
func WithStooqSymbolMapFile(path string) StooqOption {
	return func(c *StooqClient) error {
		f, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("stooq symbol map %s: %w", path, err)
		}
		defer f.Close()
		return loadSymbolMap(c, f)
	}
}

// NewStooqClient returns a GetOHLC-only Stooq client. It does not contact Stooq
// until GetOHLC. Metadata stays unimplemented on this type.
func NewStooqClient(opts ...StooqOption) (*StooqClient, error) {
	c := &StooqClient{
		baseURL:     defaultStooqBaseURL,
		httpClient:  &http.Client{Timeout: 30 * time.Second},
		limiter:     limiterFor("stooq"),
		toStooq:     make(map[string]string, len(defaultStooqUS)),
		toCanonical: make(map[string]string, len(defaultStooqUS)),
	}
	for canonical, code := range defaultStooqUS {
		if err := c.addMapping(canonical, code); err != nil {
			return nil, err
		}
	}
	for _, opt := range opts {
		if err := opt(c); err != nil {
			return nil, err
		}
	}
	if c.baseURL == "" {
		return nil, errors.New("stooq: empty base URL")
	}
	if c.httpClient == nil {
		return nil, errors.New("stooq: nil HTTP client")
	}
	return c, nil
}

func (c *StooqClient) addMapping(canonical, code string) error {
	canonical = strings.ToUpper(strings.TrimSpace(canonical))
	code = strings.ToLower(strings.TrimSpace(code))
	if canonical == "" || code == "" {
		return errors.New("stooq: empty canonical or code in symbol map")
	}
	c.toStooq[canonical] = code
	c.toCanonical[code] = canonical
	return nil
}

func loadSymbolMap(c *StooqClient, r io.Reader) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("stooq symbol map: %w", err)
	}
	for i, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := splitMapLine(line)
		if len(fields) != 2 {
			return fmt.Errorf("stooq symbol map: line %d: want canonical,code", i+1)
		}
		if i == 0 && strings.EqualFold(fields[0], "canonical") {
			continue
		}
		if err := c.addMapping(fields[0], fields[1]); err != nil {
			return fmt.Errorf("stooq symbol map: line %d: %w", i+1, err)
		}
	}
	return nil
}

func splitMapLine(line string) []string {
	if strings.Contains(line, ",") {
		parts := strings.Split(line, ",")
		out := make([]string, 0, len(parts))
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p != "" {
				out = append(out, p)
			}
		}
		return out
	}
	return strings.Fields(line)
}

// resolveSymbol maps a caller ticker to (Stooq code, canonical Yahoo-style ticker).
// Lookup is case-insensitive. A Stooq code (aapl.us) maps back to AAPL.
// The file map is applied first. Everything else is the mapping layer:
// a share class, an exchange suffix, or the client's exchange. There is
// no implied market.
func (c *StooqClient) resolveSymbol(symbol string) (code, canonical string, err error) {
	symbol = strings.TrimSpace(symbol)
	if symbol == "" {
		return "", "", errors.New("stooq: empty symbol")
	}
	if c.exchange == "" {
		return "", "", &userSentence{text: usercopy.StooqExchangeMissing}
	}
	lower := strings.ToLower(symbol)
	upper := strings.ToUpper(symbol)
	if canon, ok := c.toCanonical[lower]; ok {
		return lower, canon, nil
	}
	if mapped, ok := c.toStooq[upper]; ok {
		return mapped, upper, nil
	}
	code, canonical, err = mapping.FormatStooq(c.classes, symbol, c.exchange)
	if err != nil {
		return "", "", stooqMapErr(err)
	}
	return code, canonical, nil
}

// stooqMapErr turns a mapping-layer refusal into the sentence this client shows.
func stooqMapErr(err error) error {
	var suffix *mapping.StooqSuffixError
	if errors.As(err, &suffix) {
		return &stooqRefusal{msg: stooqExchangeRefusal(suffix.Symbol, suffix.Suffix)}
	}
	if errors.Is(err, mapping.ErrStooqTicker) {
		return errStooqInvalidTicker
	}
	return err
}

func (c *StooqClient) GetAsset(context.Context, string) (*pb.Asset, error) {
	return nil, fmt.Errorf("GetAsset: %w", ErrStooqMetadata)
}

func (c *StooqClient) SearchAsset(context.Context, string) ([]*pb.Asset, error) {
	return nil, fmt.Errorf("SearchAsset: %w", ErrStooqMetadata)
}

func (c *StooqClient) GetIndex(context.Context, string) ([]*pb.Asset, error) {
	return nil, fmt.Errorf("GetIndex: %w", ErrStooqMetadata)
}

func (c *StooqClient) GetOHLC(ctx context.Context, symbol string, start time.Time, end *time.Time) ([]*pb.OHLC, error) {
	code, canonical, err := c.resolveSymbol(symbol)
	if err != nil {
		return nil, err
	}

	now := sessionNow(ctx, c.now)
	endTime := now.UTC()
	if end != nil {
		endTime = *end
	}
	startDay := dateOnly(start)
	endDay := dateOnly(endTime)
	if endDay.Before(startDay) {
		return nil, fmt.Errorf("stooq GetOHLC %s: end before start", canonical)
	}
	endDay = stooqLastCompletedDay(endDay, now)
	if endDay.Before(startDay) {
		// Only today's unfinished session was asked for.
		return []*pb.OHLC{}, nil
	}

	reqURL, err := c.ohlcURL(code, startDay, endDay)
	if err != nil {
		return nil, fmt.Errorf("stooq GetOHLC %s: %w", canonical, err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("stooq GetOHLC %s: %w", canonical, err)
	}
	req.Header.Set("User-Agent", stooqUserAgent)

	resp, err := c.limitedClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("stooq GetOHLC %s: %w", canonical, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("stooq GetOHLC %s: read body: %w", canonical, err)
	}

	if err := blockedError("stooq", canonical, resp.Header.Get("Content-Type"), body); err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("stooq GetOHLC %s: missing file: HTTP %s", canonical, resp.Status)
	}

	bars, err := parseStooqCSV(canonical, body, startDay, endDay)
	if err != nil {
		return nil, fmt.Errorf("stooq GetOHLC %s: %w", canonical, err)
	}
	return bars, nil
}

// limitedClient sends through the shared Stooq limiter with the configured
// client's timeout and transport.
func (c *StooqClient) limitedClient() *http.Client {
	return &http.Client{
		Timeout:   c.httpClient.Timeout,
		Transport: newLimitedTransport("stooq", c.limiter, c.httpClient.Transport, rateLimitRetries),
	}
}

func (c *StooqClient) ohlcURL(code string, start, end time.Time) (string, error) {
	u, err := url.Parse(c.baseURL)
	if err != nil {
		return "", fmt.Errorf("base URL: %w", err)
	}
	u.Path = strings.TrimRight(u.Path, "/") + stooqDailyPath
	q := u.Query()
	q.Set("s", code)
	q.Set("i", "d")
	q.Set("d1", start.Format("20060102"))
	q.Set("d2", end.Format("20060102"))
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func parseStooqCSV(canonical string, body []byte, start, end time.Time) ([]*pb.OHLC, error) {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return nil, errors.New("empty response")
	}
	lower := bytes.ToLower(trimmed)
	// The daily limit is checked first: a refusal that also says "No data"
	// is a failed request, not Stooq's answer that the symbol has no bars.
	if bytes.Contains(lower, []byte("exceeded the daily hits limit")) {
		return nil, errors.New("daily hits limit exceeded")
	}
	if bytes.HasPrefix(lower, []byte("no data")) {
		return nil, noBars("missing file: no data")
	}
	reader := csv.NewReader(bytes.NewReader(trimmed))
	reader.FieldsPerRecord = -1
	reader.TrimLeadingSpace = true

	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("unparsable CSV: %w", err)
	}
	if len(records) == 0 {
		return nil, errors.New("empty response")
	}

	header := records[0]
	if !isStooqHeader(header) {
		return nil, fmt.Errorf("unparsable CSV: unexpected header %q", strings.Join(header, ","))
	}
	if len(records) == 1 {
		return nil, noBars("empty response: header only")
	}

	bars := make([]*pb.OHLC, 0, len(records)-1)
	for i, rec := range records[1:] {
		bar, err := parseStooqRow(canonical, rec, start, end)
		if err != nil {
			return nil, fmt.Errorf("unparsable CSV: row %d: %w", i+2, err)
		}
		if bar == nil {
			continue
		}
		bars = append(bars, bar)
	}
	if len(bars) == 0 {
		return nil, noBars("empty response: no rows in range")
	}
	return bars, nil
}

func isStooqHeader(fields []string) bool {
	if len(fields) < 6 {
		return false
	}
	want := []string{"date", "open", "high", "low", "close", "volume"}
	for i, w := range want {
		if !strings.EqualFold(strings.TrimSpace(fields[i]), w) {
			return false
		}
	}
	return true
}

func parseStooqRow(canonical string, rec []string, start, end time.Time) (*pb.OHLC, error) {
	if len(rec) < 6 {
		return nil, fmt.Errorf("want 6 columns, got %d", len(rec))
	}
	ts, err := time.ParseInLocation("2006-01-02", strings.TrimSpace(rec[0]), time.UTC)
	if err != nil {
		return nil, fmt.Errorf("date: %w", err)
	}
	day := dateOnly(ts)
	if day.Before(start) || afterLastSession("stooq", canonical, day, end) {
		return nil, nil
	}
	open, err := parsePrice(rec[1], "open")
	if err != nil {
		return nil, err
	}
	high, err := parsePrice(rec[2], "high")
	if err != nil {
		return nil, err
	}
	low, err := parsePrice(rec[3], "low")
	if err != nil {
		return nil, err
	}
	closePx, err := parsePrice(rec[4], "close")
	if err != nil {
		return nil, err
	}
	vol, err := parseVolume(rec[5])
	if err != nil {
		return nil, err
	}
	return &pb.OHLC{
		Symbol:    canonical,
		Timestamp: timestamppb.New(day),
		Open:      open,
		High:      high,
		Low:       low,
		Close:     closePx,
		Volume:    vol,
	}, nil
}

func parsePrice(s, field string) (float64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty %s", field)
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", field, err)
	}
	return v, nil
}

func parseVolume(s string) (uint64, error) {
	s = strings.TrimSpace(s)
	if s == "" || s == "-" {
		return 0, nil
	}
	v, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("volume: %w", err)
	}
	return v, nil
}

// stooqLastCompletedDay clamps Stooq's inclusive end day so that today's
// unfinished XNYS session is never asked for. The day ends at midnight New
// York after it; if lastCompletedSessionEnd stops the fetch before that,
// the last day is the New York date just before the limit. A symbol on
// another exchange is clamped by the XNYS close too, which can leave a
// finished London or Xetra session out until 16:30 New York.
func stooqLastCompletedDay(endDay, now time.Time) time.Time {
	year, month, day := endDay.Date()
	dayEnd := time.Date(year, month, day+1, 0, 0, 0, 0, xnysZone)
	limit := lastCompletedSessionEnd(dayEnd, now)
	if !limit.Before(dayEnd) {
		return endDay
	}
	year, month, day = limit.Add(-time.Nanosecond).In(xnysZone).Date()
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}

func dateOnly(t time.Time) time.Time {
	y, m, d := t.UTC().Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}
