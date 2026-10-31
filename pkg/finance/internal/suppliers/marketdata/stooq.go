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
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

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
// Canonical form (AAPL) is what Postgres/ohlc.symbol stores. Unknown tickers without
// an exchange suffix use the documented fallback lower(symbol)+".us".
var defaultStooqUS = map[string]string{
	"AAPL":  "aapl.us",
	"SPY":   "spy.us",
	"MSFT":  "msft.us",
	"GOOGL": "googl.us",
	"NVDA":  "nvda.us",
}

// stooqExchangeSuffix maps an exchange suffix, as written in a stored symbol
// (Yahoo style, such as VOD.L), to Stooq's suffix for the same exchange.
// Stooq's own suffix (VOD.UK) is accepted too. A suffix that is in neither
// form is refused with ErrStooqExchange instead of guessing a code. The keys
// are the suffixes the refusal lists as supported.
var stooqExchangeSuffix = map[string]string{
	"L":  "uk", // London Stock Exchange
	"DE": "de", // Xetra
	"US": "us",
}

// stooqSuffixFor returns Stooq's suffix for a Yahoo-style or Stooq-native
// exchange suffix.
func stooqSuffixFor(suffix string) (string, bool) {
	if code, ok := stooqExchangeSuffix[strings.ToUpper(suffix)]; ok {
		return code, true
	}
	lower := strings.ToLower(suffix)
	for _, code := range stooqExchangeSuffix {
		if code == lower {
			return code, true
		}
	}
	return "", false
}

// stooqSupportedSuffixes lists the supported suffixes as ".DE, .L, .US".
func stooqSupportedSuffixes() string {
	suffixes := make([]string, 0, len(stooqExchangeSuffix))
	for suffix := range stooqExchangeSuffix {
		suffixes = append(suffixes, "."+suffix)
	}
	sort.Strings(suffixes)
	return strings.Join(suffixes, ", ")
}

// stooqRefusal is the user-facing text for a symbol Stooq can't be asked
// for. It matches ErrStooqExchange with errors.Is.
type stooqRefusal struct{ msg string }

func (e *stooqRefusal) Error() string { return e.msg }
func (e *stooqRefusal) Unwrap() error { return ErrStooqExchange }

// errStooqInvalidTicker never echoes the symbol: it failed the bound check.
var errStooqInvalidTicker = &stooqRefusal{
	msg: "stooq can't use this symbol because it isn't a valid ticker. Check the spelling, for example VOD.L or AAPL.",
}

// stooqTicker bounds a suffixed ticker before it is echoed in an error, so
// a long or control-character symbol never reaches a log line or the CLI.
var stooqTicker = regexp.MustCompile(`^[A-Z0-9^=_-]{1,32}\.[A-Z]{1,8}$`)

// ErrStooqExchange is returned by GetOHLC for a symbol whose exchange suffix
// has no known Stooq code.
var ErrStooqExchange = errors.New("stooq has no code for this exchange")

var _ supplier.Marketdata = (*StooqClient)(nil)

// StooqClient fetches daily OHLC from Stooq CSV. Metadata methods do not scrape Stooq.
type StooqClient struct {
	baseURL     string
	httpClient  *http.Client
	toStooq     map[string]string // canonical upper → stooq code
	toCanonical map[string]string // stooq code lower → canonical upper
}

// StooqOption configures a StooqClient.
type StooqOption func(*StooqClient) error

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
// Unknown symbols without a '.' suffix use lower(symbol)+".us"; a symbol with
// an exchange suffix goes through exchangeCode.
func (c *StooqClient) resolveSymbol(symbol string) (code, canonical string, err error) {
	symbol = strings.TrimSpace(symbol)
	if symbol == "" {
		return "", "", errors.New("stooq: empty symbol")
	}
	lower := strings.ToLower(symbol)
	upper := strings.ToUpper(symbol)
	if canon, ok := c.toCanonical[lower]; ok {
		return lower, canon, nil
	}
	if mapped, ok := c.toStooq[upper]; ok {
		return mapped, upper, nil
	}
	if code, ok := stooqShareClassCode(symbol); ok {
		return code, upper, nil
	}
	if i := strings.LastIndexByte(symbol, '.'); i > 0 {
		return exchangeCode(symbol[:i], symbol[i+1:])
	}
	return lower + ".us", upper, nil
}

// exchangeCode maps a ticker with an exchange suffix (VOD.L) to its Stooq
// code (vod.uk). Bars keep the symbol as asked (VOD.L), so they are stored
// under the name the caller stores. A US code (xyz.us) is labelled XYZ, the
// form US tickers are stored under.
func exchangeCode(ticker, suffix string) (code, canonical string, err error) {
	upper := strings.ToUpper(ticker + "." + suffix)
	if !stooqTicker.MatchString(upper) {
		return "", "", errStooqInvalidTicker
	}
	stooqSuffix, ok := stooqSuffixFor(suffix)
	if !ok {
		return "", "", &stooqRefusal{msg: fmt.Sprintf(
			"stooq doesn't support the .%s exchange (%s). Supported suffixes: %s. Try another provider with --provider.",
			strings.ToUpper(suffix), upper, stooqSupportedSuffixes())}
	}
	code = strings.ToLower(ticker) + "." + stooqSuffix
	if stooqSuffix == "us" {
		return code, strings.ToUpper(ticker), nil
	}
	return code, upper, nil
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

	endTime := time.Now().UTC()
	if end != nil {
		endTime = *end
	}
	startDay := dateOnly(start)
	endDay := dateOnly(endTime)
	if endDay.Before(startDay) {
		return nil, fmt.Errorf("stooq GetOHLC %s: end before start", canonical)
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

	resp, err := c.httpClient.Do(req)
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
	if bytes.HasPrefix(lower, []byte("no data")) {
		return nil, errors.New("missing file: no data")
	}
	if bytes.Contains(lower, []byte("exceeded the daily hits limit")) {
		return nil, errors.New("daily hits limit exceeded")
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
		return nil, errors.New("empty response: header only")
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
		return nil, errors.New("empty response: no rows in range")
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
	if day.Before(start) || day.After(end) {
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

func dateOnly(t time.Time) time.Time {
	y, m, d := t.UTC().Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}
