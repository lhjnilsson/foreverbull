package marketdata

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/lhjnilsson/foreverbull/pkg/finance/supplier"
	pb "github.com/lhjnilsson/foreverbull/pkg/pb/finance"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	bufferSize = 128
)

// defaultYahooChartURL is the chart endpoint; the symbol is appended to it.
const defaultYahooChartURL = "https://query2.finance.yahoo.com/v8/finance/chart/"

// defaultYahooQuoteURL is the quote summary endpoint; the symbol is appended to it.
const defaultYahooQuoteURL = "https://query2.finance.yahoo.com/v10/finance/quoteSummary/"

// defaultYahooSearchURL is the asset search endpoint; the query goes in q.
const defaultYahooSearchURL = "https://query2.finance.yahoo.com/v1/finance/search"

// defaultISINSearchURL is the ISIN lookup; the symbol is appended to it.
const defaultISINSearchURL = "https://markets.businessinsider.com/ajax/markets-insider/search/"

// yahooHTTPTimeout bounds every Yahoo and ISIN request, so a provider that
// stops answering fails the symbol instead of hanging the download.
const yahooHTTPTimeout = 30 * time.Second

// isinBodyLimit caps the ISIN search answer read into memory.
const isinBodyLimit = 1 << 20

// yahooHTTPClient is shared by every YahooClient that does not set its own.
var yahooHTTPClient = &http.Client{Timeout: yahooHTTPTimeout}

type YahooClient struct {
	cookie string
	crumb  string
	// chartURL, quoteURL, isinURL and searchURL replace the defaults so unit
	// tests stay off the network.
	chartURL  string
	quoteURL  string
	isinURL   string
	searchURL string
	// httpClient replaces the shared client, so tests can use a short timeout.
	httpClient *http.Client
	// now replaces the clock, so tests can fix today's session.
	now func() time.Time
}

func (y *YahooClient) client() *http.Client {
	if y.httpClient != nil {
		return y.httpClient
	}
	return yahooHTTPClient
}

func NewYahooClient() (supplier.Marketdata, error) {
	yahoo := &YahooClient{}

	rsp, err := yahoo.doRequest("https://fc.yahoo.com")
	if err != nil {
		return nil, fmt.Errorf("error making request: %w", err)
	}

	defer rsp.Body.Close()

	yahoo.cookie = rsp.Header.Get("Set-Cookie")

	rsp, err = yahoo.doRequest("https://query2.finance.yahoo.com/v1/test/getcrumb")
	if err != nil {
		return nil, err
	}
	defer rsp.Body.Close()

	buffer := make([]byte, bufferSize)

	length, err := rsp.Body.Read(buffer)
	if err != nil {
		return nil, fmt.Errorf("error reading response: %w", err)
	}

	yahoo.crumb = string(buffer[:length])

	return yahoo, nil
}

func (y *YahooClient) doRequest(rawURL string, params ...string) (*http.Response, error) {
	// supplier.Marketdata carries no context, so there is no request ctx to
	// pass here; the client timeout bounds the request instead.
	reqURL, err := requestURL(rawURL, y.crumb, params...)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(context.TODO(), http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	req.Header.Add("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36")

	if y.cookie != "" {
		req.Header.Add("Cookie", y.cookie)
	}

	rsp, err := y.client().Do(req)
	if err != nil {
		return nil, fmt.Errorf("error making request: %w", err)
	}
	return rsp, nil
}

type AssetResponse struct {
	QuoteSummary struct {
		Result []struct {
			AssetProfile struct{} `json:"assetProfile"`
			QuoteType    struct {
				Exchange  string `json:"exchange"`
				Symbol    string `json:"symbol"`
				QuoteType string `json:"quoteType"`
				ShortName string `json:"shortName"`
				LongName  string `json:"longName"`
			} `json:"quoteType"`
		} `json:"result"`
		Error struct {
			Code        string `json:"code"`
			Description string `json:"description"`
		} `json:"error"`
	} `json:"quoteSummary"`
}

func (y *YahooClient) getISIN(symbol string) (string, error) {
	isinURL := y.isinURL
	if isinURL == "" {
		isinURL = defaultISINSearchURL
	}
	req, err := http.NewRequestWithContext(context.TODO(), http.MethodGet, isinURL+url.PathEscape(symbol), nil)
	if err != nil {
		return "", fmt.Errorf("error creating request: %w", err)
	}
	resp, err := y.client().Do(req)
	if err != nil {
		return "", fmt.Errorf("error making HTTP request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to get index data for symbol %s: status %s", symbol, resp.Status)
	}

	var data map[string]interface{}
	if err := json.NewDecoder(io.LimitReader(resp.Body, isinBodyLimit)).Decode(&data); err != nil {
		return "", fmt.Errorf("error decoding response: %w", err)
	}

	stocks, ok := data["stocks"]
	if !ok {
		return "", fmt.Errorf("response missing 'stocks' field")
	}
	stocksSlice, ok := stocks.([]interface{})
	if !ok {
		return "", fmt.Errorf("'stocks' field is not an array")
	}

	for _, stock := range stocksSlice {
		stockData, ok := stock.(map[string]interface{})
		if !ok {
			continue // Skip invalid stock entries
		}
		keywords, ok := stockData["keywords"].(string)
		if !ok {
			continue
		}

		parts := strings.Split(keywords, "|")
		if len(parts) < 2 {
			return "", fmt.Errorf("invalid keywords format for stock %s", symbol)
		}
		if strings.Compare(parts[0], symbol) == 0 {
			return parts[1], nil
		}
	}
	return "", noISIN(fmt.Sprintf("ISIN not found for symbol %s", symbol))
}

// GetAsset looks the symbol up under Yahoo's ticker (BRK-B for BRKB). A
// share class is returned under the asked ticker, so it is stored and
// reported the way the universe names it.
func (y *YahooClient) GetAsset(symbol string) (*pb.Asset, error) {
	quoteURL := y.quoteURL
	if quoteURL == "" {
		quoteURL = defaultYahooQuoteURL
	}
	resp, err := y.doRequest(quoteURL+url.PathEscape(yahooSymbol(symbol)), "modules=quoteType")
	if err != nil {
		return nil, fmt.Errorf("error: %w", err)
	}
	defer resp.Body.Close()

	data := AssetResponse{}

	err = json.NewDecoder(resp.Body).Decode(&data)
	if err != nil {
		return nil, fmt.Errorf("error decoding response: %w", err)
	}

	if data.QuoteSummary.Error.Code != "" {
		return nil, fmt.Errorf("%v", data.QuoteSummary.Error.Description)
	}

	if len(data.QuoteSummary.Result) == 0 {
		return nil, fmt.Errorf("no results found")
	}

	asset := pb.Asset{
		Symbol: data.QuoteSummary.Result[0].QuoteType.Symbol,
		Name:   data.QuoteSummary.Result[0].QuoteType.LongName,
		ISIN:   "",
	}

	// An equity is matched across renames by its ISIN, so an equity without
	// one fails the lookup instead of being stored without it. Other quote
	// types (ETF, index, fund) have no ISIN lookup.
	if data.QuoteSummary.Result[0].QuoteType.QuoteType == "EQUITY" {
		asset.ISIN, err = y.getISIN(asset.Symbol)
		if err != nil {
			return nil, fmt.Errorf("error getting isin for symbol %s: %w", asset.Symbol, err)
		}
		if strings.TrimSpace(asset.ISIN) == "" {
			return nil, fmt.Errorf("error getting isin for symbol %s: %w", asset.Symbol, noISIN("empty ISIN"))
		}
	}
	if _, ok := parseShareClass(symbol); ok {
		asset.Symbol = strings.ToUpper(strings.TrimSpace(symbol))
	}

	return &asset, nil
}

func (m *YahooClient) SearchAsset(q string) ([]*pb.Asset, error) {
	resp, err := m.doRequest(m.searchRequestURL(q))
	if err != nil {
		return nil, fmt.Errorf("error: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fail to search asset: %s", resp.Status)
	}
	var data map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&data)
	if err != nil {
		return nil, fmt.Errorf("error decoding response: %w", err)
	}

	assets := make([]*pb.Asset, 0)
	for _, quote := range data["quotes"].([]interface{}) {
		quoteMap := quote.(map[string]interface{})
		symbol := quoteMap["symbol"].(string)
		a, err := m.GetAsset(symbol)
		if errors.Is(err, ErrNoISIN) {
			// An equity without an ISIN can't match the search by
			// identity, so it is a known non-match, not a failure.
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("error getting asset: %w", err)
		}
		assets = append(assets, a)
	}
	return assets, nil
}

// requestURL adds params (already encoded "k=v" pairs) and the crumb to the
// query of rawURL, keeping any query it already has. Appending "?crumb=" to a
// URL that already had a query hid the crumb inside the previous value.
// An unparsable rawURL is an error. It names only the parse failure, never
// the URL, so no query value (or crumb) reaches an error or a log line.
func requestURL(rawURL, crumb string, params ...string) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		var urlErr *url.Error
		if errors.As(err, &urlErr) {
			err = urlErr.Err
		}
		return "", fmt.Errorf("invalid request URL: %w", err)
	}
	parts := make([]string, 0, len(params)+2)
	if u.RawQuery != "" {
		parts = append(parts, u.RawQuery)
	}
	if crumb != "" {
		parts = append(parts, "crumb="+url.QueryEscape(crumb))
	}
	parts = append(parts, params...)
	u.RawQuery = strings.Join(parts, "&")
	return u.String(), nil
}

// searchRequestURL is the search URL with q escaped as one query value, so
// `&`, `?`, `#` or a space in q cannot add parameters or cut the URL short.
func (m *YahooClient) searchRequestURL(q string) string {
	base := m.searchURL
	if base == "" {
		base = defaultYahooSearchURL
	}
	params := url.Values{}
	params.Set("q", q)
	params.Set("lang", "en-US")
	params.Set("region", "US")
	params.Set("quotesCount", "6")
	params.Set("newsCount", "0")
	params.Set("listsCount", "0")
	params.Set("quotesQueryId", "tss_match_phrase_query")
	return base + "?" + params.Encode()
}

type IndexResponse struct {
	Summary struct {
		Result []struct {
			Components struct {
				Components []string `json:"components"`
			} `json:"components"`
		} `json:"result"`
	} `json:"quoteSummary"`
}

// GetIndex returns the index's member assets followed by the index asset.
// Every member is looked up; if any lookup fails the index fails, and the
// error names each member that failed.
func (y *YahooClient) GetIndex(symbol string) ([]*pb.Asset, error) {
	quoteURL := y.quoteURL
	if quoteURL == "" {
		quoteURL = defaultYahooQuoteURL
	}
	resp, err := y.doRequest(quoteURL+url.PathEscape(strings.ToUpper(symbol)), "modules=components%2CsummaryDetail")
	if err != nil {
		return nil, fmt.Errorf("error: %w", err)
	}

	result := IndexResponse{}

	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fail to get index data for symbol %s: %s", symbol, resp.Status)
	}

	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return nil, fmt.Errorf("error decoding response: %w", err)
	}
	if len(result.Summary.Result) == 0 {
		return nil, fmt.Errorf("fail to get index data for symbol %s: no result", symbol)
	}

	components := result.Summary.Result[0].Components.Components
	// Each lookup writes only its own slot, so all results are in once
	// Wait returns and no slot is shared between goroutines.
	assets := make([]*pb.Asset, len(components)+1)
	errs := make([]error, len(components)+1)
	var wg sync.WaitGroup
	lookup := func(i int, name, kind string) {
		defer wg.Done()
		a, err := y.GetAsset(name)
		if err == nil && a == nil {
			err = errors.New("no asset returned")
		}
		if err != nil {
			errs[i] = fmt.Errorf("%s %s: %w", kind, name, err)
			return
		}
		assets[i] = a
	}
	for i, component := range components {
		wg.Add(1)
		go lookup(i, component, "component")
	}
	wg.Add(1)
	go lookup(len(components), symbol, "index")
	wg.Wait()

	if err := errors.Join(errs...); err != nil {
		return nil, fmt.Errorf("error getting assets: %w", err)
	}
	return assets, nil
}

// yahooQuote holds pointers because Yahoo sends null for a session it has
// no price for, and a float64 would decode that null as a 0 price.
type yahooQuote struct {
	Open   []*float64 `json:"open"`
	High   []*float64 `json:"high"`
	Low    []*float64 `json:"low"`
	Close  []*float64 `json:"close"`
	Volume []*uint64  `json:"volume"`
}

type OHLCResponse struct {
	Chart struct {
		Result []struct {
			Meta struct {
				Currency string `json:"currency"`
				Symbol   string `json:"symbol"`
			} `json:"meta"`
			Timestamp  []int64 `json:"timestamp"`
			Indicators struct {
				Quote []yahooQuote `json:"quote"`
			} `json:"indicators"`
		} `json:"result"`
		Error struct {
			Code        string `json:"code"`
			Description string `json:"description"`
		}
	}
}

// yahooChartNotFound is the chart error code for a ticker Yahoo has no
// bars for ("No data found, symbol may be delisted").
const yahooChartNotFound = "Not Found"

func (y *YahooClient) GetOHLC(symbol string, start time.Time, end *time.Time) ([]*pb.OHLC, error) {
	chartURL := y.chartURL
	if chartURL == "" {
		chartURL = defaultYahooChartURL
	}
	chart := chartURL + url.PathEscape(yahooSymbol(symbol))
	params := []string{}
	params = append(params, fmt.Sprintf("period1=%d", start.Unix()))

	now := clockNow(y.now)
	until := now
	if end != nil {
		// period2 is exclusive, so the end date's bar needs the next
		// midnight New York.
		until = inclusiveFetchEnd(*end)
	}
	// Today's unfinished session is the only one left out, by not asking
	// for it. Every bar in the answer must then have a price.
	until, ok := completedFetchEnd(start, until, now)
	if !ok {
		return []*pb.OHLC{}, nil
	}
	params = append(params, fmt.Sprintf("period2=%d", until.Unix()))

	params = append(params, "interval=1d")

	resp, err := y.doRequest(chart, params...)
	if err != nil {
		return nil, fmt.Errorf("fail to get OHLC data for symbol %s: %w", symbol, err)
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response: %w", err)
	}
	if err := blockedError("yahoo", strings.ToUpper(symbol), resp.Header.Get("Content-Type"), body); err != nil {
		return nil, err
	}
	// Yahoo answers an unknown ticker with 404 and a "Not Found" chart error.
	// Any other non-200 is a failed request, whatever the body says.
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotFound {
		return nil, fmt.Errorf("fail to get OHLC data for symbol %s: HTTP %s", symbol, resp.Status)
	}

	data := OHLCResponse{}
	err = json.Unmarshal(body, &data)
	if err != nil {
		return nil, fmt.Errorf("error decoding response: %w", err)
	}

	if data.Chart.Error.Code != "" {
		description := errors.New(data.Chart.Error.Description)
		if data.Chart.Error.Code == yahooChartNotFound {
			// Yahoo answered: it has no chart for this ticker.
			description = noBars(data.Chart.Error.Description)
		}
		return nil, fmt.Errorf("fail to get OHLC data for symbol %s: %w", symbol, description)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fail to get OHLC data for symbol %s: HTTP %s", symbol, resp.Status)
	}

	if len(data.Chart.Result) == 0 {
		return nil, fmt.Errorf("fail to get OHLC data for symbol %s: no result", symbol)
	}
	timestamps, quotes := yahooThrough(strings.ToUpper(symbol),
		data.Chart.Result[0].Timestamp, data.Chart.Result[0].Indicators.Quote, lastFetchedSession(end, now))
	ohlcs, err := yahooBars(timestamps, quotes)
	if err != nil {
		var missing *MissingPriceError
		if errors.As(err, &missing) {
			missing.Provider = "yahoo"
			missing.Symbol = strings.ToUpper(symbol)
			return nil, missing
		}
		return nil, fmt.Errorf("fail to get OHLC data for symbol %s: %w", symbol, err)
	}
	return ohlcs, nil
}

// yahooBars turns Yahoo's column arrays into bars. A session where any of
// open, high, low, or close is null, non-finite, or not positive is a
// missing bar, wherever it is in the answer: it fails with a
// MissingPriceError naming every such day, so no 0 price reaches storage
// and the stored bars have no gap. GetOHLC never asks for today's unfinished
// session and drops it if Yahoo sends it anyway, so it is not one. A null
// volume on a priced session is 0 shares.
// Arrays of different lengths are refused rather than read out of range.
func yahooBars(timestamps []int64, quotes []yahooQuote) ([]*pb.OHLC, error) {
	if len(timestamps) == 0 {
		return []*pb.OHLC{}, nil
	}
	if len(quotes) == 0 {
		return nil, fmt.Errorf("no quote for %d timestamps", len(timestamps))
	}
	quote := quotes[0]
	barCount := len(timestamps)
	columns := []struct {
		name  string
		count int
	}{
		{"open", len(quote.Open)}, {"high", len(quote.High)}, {"low", len(quote.Low)},
		{"close", len(quote.Close)}, {"volume", len(quote.Volume)},
	}
	for _, column := range columns {
		if column.count != barCount {
			return nil, fmt.Errorf("%s has %d values for %d timestamps", column.name, column.count, barCount)
		}
	}

	ohlcs := make([]*pb.OHLC, 0, barCount)
	var missing []time.Time
	for index, ts := range timestamps {
		day := time.Unix(ts, 0)
		open, high, low, closePx := quote.Open[index], quote.High[index], quote.Low[index], quote.Close[index]
		if !yahooPrice(open) || !yahooPrice(high) || !yahooPrice(low) || !yahooPrice(closePx) {
			missing = append(missing, day)
			continue
		}
		var volume uint64
		if quote.Volume[index] != nil {
			volume = *quote.Volume[index]
		}
		ohlcs = append(ohlcs, &pb.OHLC{
			Timestamp: timestamppb.New(day),
			Open:      *open,
			High:      *high,
			Low:       *low,
			Close:     *closePx,
			Volume:    volume,
		})
	}
	if len(missing) > 0 {
		return nil, &MissingPriceError{Days: missing}
	}
	return ohlcs, nil
}

// yahooThrough drops every session Yahoo sent after last, the end date or
// the last completed session when that is earlier (lastFetchedSession),
// before any price is checked. Yahoo's chart API
// sometimes sends today's live bar even when period2 is earlier; it is
// unfinished, so it is neither stored nor, with its null close, a missing
// price. Columns of a length other than the timestamps' are left to
// yahooBars to refuse.
func yahooThrough(symbol string, timestamps []int64, quotes []yahooQuote, last time.Time) ([]int64, []yahooQuote) {
	if len(quotes) == 0 {
		return timestamps, quotes
	}
	quote := quotes[0]
	count := len(timestamps)
	if len(quote.Open) != count || len(quote.High) != count || len(quote.Low) != count ||
		len(quote.Close) != count || len(quote.Volume) != count {
		return timestamps, quotes
	}
	kept := make([]int64, 0, count)
	trimmed := yahooQuote{
		Open:   make([]*float64, 0, count),
		High:   make([]*float64, 0, count),
		Low:    make([]*float64, 0, count),
		Close:  make([]*float64, 0, count),
		Volume: make([]*uint64, 0, count),
	}
	for index, ts := range timestamps {
		if afterLastSession("yahoo", symbol, time.Unix(ts, 0).In(xnysZone), last) {
			continue
		}
		kept = append(kept, ts)
		trimmed.Open = append(trimmed.Open, quote.Open[index])
		trimmed.High = append(trimmed.High, quote.High[index])
		trimmed.Low = append(trimmed.Low, quote.Low[index])
		trimmed.Close = append(trimmed.Close, quote.Close[index])
		trimmed.Volume = append(trimmed.Volume, quote.Volume[index])
	}
	return kept, []yahooQuote{trimmed}
}

// yahooPrice is true for a real positive price.
func yahooPrice(price *float64) bool {
	if price == nil {
		return false
	}
	return *price > 0 && !math.IsNaN(*price) && !math.IsInf(*price, 0)
}
