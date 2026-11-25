package marketdata

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestClassifySuffixWithQuote(t *testing.T) {
	got, ok, err := Classify("yahoo", "VOD.L", Quote{Exchange: "LSE", Currency: "GBP", Reported: true}, nil)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "London", got.Place)
	require.Equal(t, "GBP", got.Currency)
	require.Equal(t, "yahoo", got.Provider)
	require.False(t, got.CurrencyOnly)
}

func TestClassifyCurrencyWithoutSuffix(t *testing.T) {
	got, ok, err := Classify("yahoo", "XYZ", Quote{Exchange: "NMS", Currency: "CAD", Reported: true}, nil)
	require.NoError(t, err)
	require.True(t, ok)
	require.True(t, got.CurrencyOnly)
	require.Equal(t, "CAD", got.Currency)
	require.Equal(t, "yahoo", got.Provider)
	require.Empty(t, got.Place)
}

func TestClassifySuffixWithoutQuote(t *testing.T) {
	got, ok, err := Classify("stooq", "VOD.L", Quote{}, nil)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "London", got.Place)
	require.Empty(t, got.Currency)
	require.False(t, got.CurrencyOnly)

	unknown, ok, err := Classify("stooq", "ABC.XX", Quote{}, nil)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "XX", unknown.Suffix)
	require.Empty(t, unknown.Place)
}

func TestClassifyUnknownExchangeIsRefused(t *testing.T) {
	got, ok, err := Classify("yahoo", "XYZ", Quote{Exchange: "ZZZ", Currency: "USD", Reported: true}, nil)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "ZZZ", got.Place)
}

func TestClassifyUnknownSymbolIsNotARefusal(t *testing.T) {
	_, ok, err := Classify("yahoo", "NOPE", Quote{}, ErrUnknownSymbol)
	require.NoError(t, err)
	require.False(t, ok)
}

func TestClassifyUSQuoteIsAccepted(t *testing.T) {
	_, ok, err := Classify("yahoo", "AAPL", Quote{Exchange: "NMS", Currency: "USD", Reported: true}, nil)
	require.NoError(t, err)
	require.False(t, ok)
	_, ok, err = Classify("yahoo", "BRK.B", Quote{Exchange: "NYQ", Currency: "USD", Reported: true}, nil)
	require.NoError(t, err)
	require.False(t, ok, "a share class is not an exchange suffix")
	_, ok, err = Classify("stooq", "AAPL.US", Quote{}, nil)
	require.NoError(t, err)
	require.False(t, ok)
}

func TestClassifySuffixRefusesDespiteAUSQuote(t *testing.T) {
	got, ok, err := Classify("yahoo", "VOD.L", Quote{Exchange: "NMS", Currency: "USD", Reported: true}, nil)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "London", got.Place)
	require.Empty(t, got.Currency)
}

func TestClassifySuffixRefusesWhenTheQuoteFails(t *testing.T) {
	got, ok, err := Classify("yahoo", "VOD.L", Quote{}, context.DeadlineExceeded)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "London", got.Place)
}

func TestClassifyQuoteFailureWithoutASuffix(t *testing.T) {
	_, ok, err := Classify("yahoo", "XYZ", Quote{}, context.DeadlineExceeded)
	require.False(t, ok)
	var failed *QuoteFailure
	require.ErrorAs(t, err, &failed)
	require.Equal(t, "XYZ", failed.Symbol)
	require.Equal(t, "yahoo", failed.Provider)
}

func TestYahooListingReadsExchangeAndCurrency(t *testing.T) {
	var path, query string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		query = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"quoteSummary":{"result":[{"quoteType":{"exchange":"LSE","symbol":"VOD.L"},"price":{"currency":"GBp","exchange":"LSE"}}]}}`))
	}))
	t.Cleanup(srv.Close)
	client := &YahooClient{crumb: "test", quoteURL: srv.URL + "/"}
	exchange, currency, err := client.Listing(context.Background(), "VOD.L")
	require.NoError(t, err)
	require.Contains(t, path, "VOD.L")
	require.Contains(t, query, "modules=quoteType,price")
	require.Equal(t, "LSE", exchange)
	require.Equal(t, "GBp", currency)

	got, ok, err := Classify("yahoo", "VOD.L", Quote{Exchange: exchange, Currency: currency, Reported: true}, nil)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "London", got.Place)
	require.Equal(t, "GBP", got.Currency)
}

func TestUSIndexBenchmarksPassThroughYahoo(t *testing.T) {
	for _, exchange := range []string{"NIM", "SNP", "DJI", "WCB", "CXI", "OQB", "OQX", "OEM"} {
		t.Run(exchange, func(t *testing.T) {
			symbol := "IDX" + exchange
			body := fmt.Sprintf(`{"quoteSummary":{"result":[{"quoteType":{"exchange":%q,"symbol":%q,"quoteType":"INDEX"},"price":{"currency":"USD","exchange":%q,"symbol":%q}}]}}`,
				exchange, symbol, exchange, symbol)
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(body))
			}))
			t.Cleanup(srv.Close)
			client := &YahooClient{crumb: "test", quoteURL: srv.URL + "/"}
			gotExchange, currency, err := client.Listing(context.Background(), symbol)
			require.NoError(t, err)
			require.Equal(t, exchange, gotExchange)
			require.Equal(t, "USD", currency)
			_, refused, err := Classify("yahoo", symbol, Quote{Exchange: gotExchange, Currency: currency, Reported: true}, nil)
			require.NoError(t, err)
			require.False(t, refused)
		})
	}
}

func TestYahooListingEmptyAnswerIsAnError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"quoteSummary":{"result":[{"quoteType":{"symbol":"XYZ"},"price":{"symbol":"XYZ"}}]}}`))
	}))
	t.Cleanup(srv.Close)
	client := &YahooClient{crumb: "test", quoteURL: srv.URL + "/"}
	_, _, err := client.Listing(context.Background(), "XYZ")
	require.EqualError(t, err, "yahoo listing for XYZ has no exchange")
}

func TestAlpacaListingAcceptsAUSStock(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"b0b6dd9d-8b9b-48a9-ba46-b9d54906e415","class":"us_equity","exchange":"NASDAQ","symbol":"AAPL","name":"Apple Inc.","status":"active","tradable":true,"marginable":true,"shortable":true,"easy_to_borrow":true,"fractionable":true,"currency":"USD"}`))
	}))
	t.Cleanup(srv.Close)
	bars := &AlpacaBars{tradingURL: srv.URL}
	bars.opts.APIKey = "key"
	bars.opts.APISecret = "secret"
	exchange, currency, err := bars.Listing(context.Background(), "AAPL")
	require.NoError(t, err)
	require.Equal(t, "NASDAQ", exchange)
	require.Equal(t, "USD", currency)
	_, ok, err := Classify("alpaca", "AAPL", Quote{Exchange: exchange, Currency: currency, Reported: true}, nil)
	require.NoError(t, err)
	require.False(t, ok)
}

func TestRefuseListedAsksEachSymbolOnce(t *testing.T) {
	var calls []string
	restore := SetQuoteReader(func(_ context.Context, _, _, symbol string) (Quote, error) {
		calls = append(calls, symbol)
		return Quote{Exchange: "NMS", Currency: "USD", Reported: true}, nil
	})
	defer restore()
	listed, bench, err := RefuseListed(context.Background(), "yahoo", "", []string{"MSFT", "AAPL"}, "SPY")
	require.NoError(t, err)
	require.Empty(t, listed)
	require.Nil(t, bench)
	require.Equal(t, []string{"SPY", "MSFT", "AAPL"}, calls)
	listed, bench, err = RefuseListed(context.Background(), "yahoo", "", []string{"AAPL", "MSFT"}, "SPY")
	require.NoError(t, err)
	require.Empty(t, listed)
	require.Nil(t, bench)
	require.Equal(t, []string{"SPY", "MSFT", "AAPL"}, calls)
}

func TestAlpacaListingReadsExchangeAndCurrency(t *testing.T) {
	var path string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"exchange":"TSX","currency":"CAD","status":"active","class":"us_equity"}`))
	}))
	t.Cleanup(srv.Close)
	bars := &AlpacaBars{tradingURL: srv.URL}
	bars.opts.APIKey = "key"
	bars.opts.APISecret = "secret"
	exchange, currency, err := bars.Listing(context.Background(), "SHOP")
	require.NoError(t, err)
	require.Contains(t, path, "/v2/assets/SHOP")
	require.Equal(t, "TSX", exchange)
	require.Equal(t, "CAD", currency)

	got, ok, err := Classify("alpaca", "SHOP", Quote{Exchange: exchange, Currency: currency, Reported: true}, nil)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "Toronto", got.Place)
	require.Equal(t, "CAD", got.Currency)
	require.False(t, got.CurrencyOnly, "a non-US exchange and a currency use the listing description")
}

// Recorded Yahoo quote summaries. The exchange code is the signal.
func recordedYahooQuote(symbol, exchange string) string {
	return fmt.Sprintf(`{"quoteSummary":{"result":[{"quoteType":{"exchange":%q,"symbol":%q,"quoteType":"EQUITY","shortName":%q},"price":{"currency":"USD","exchange":%q,"symbol":%q}}]}}`,
		exchange, symbol, symbol, exchange, symbol)
}

func templateExchange(symbol string) string {
	switch strings.ToUpper(strings.TrimSpace(symbol)) {
	case "^NDX":
		return "NIM"
	case "^GSPC":
		return "SNP"
	case "^DJI":
		return "DJI"
	case "^RUT":
		return "WCB"
	case "SPY", "QQQ", "OEF":
		return "PCX"
	case "BRK.B", "BRK-B", "BRKB", "JPM", "XOM":
		return "NYQ"
	case "FNMA", "FMCC":
		return "PNK"
	default:
		return "NMS"
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	require.NoError(t, err)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		require.NotEqual(t, parent, dir)
		dir = parent
	}
}

func templateSymbols(t *testing.T, path string) []string {
	t.Helper()
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	if strings.HasSuffix(path, ".json") {
		var doc struct {
			Symbols   []string `json:"symbols"`
			Benchmark string   `json:"benchmark"`
		}
		require.NoError(t, json.Unmarshal(raw, &doc))
		out := append([]string{}, doc.Symbols...)
		if doc.Benchmark != "" {
			out = append(out, doc.Benchmark)
		}
		require.NotEmpty(t, out)
		return out
	}
	var out []string
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "benchmark:"):
			out = append(out, strings.Trim(strings.TrimPrefix(line, "benchmark:"), ` "'`))
		case strings.HasPrefix(line, "- symbol:"):
			out = append(out, strings.Trim(strings.TrimPrefix(line, "- symbol:"), ` "'`))
		}
	}
	require.NotEmpty(t, out, path)
	return out
}

func TestShippedTemplatesPassRecordedYahooListings(t *testing.T) {
	root := repoRoot(t)
	files := []string{
		filepath.Join(root, "client/cli/src/foreverbull_cli/nasdaq.json"),
		filepath.Join(root, "client/templates/simple/{{cookiecutter.project_slug}}/backtest.yaml"),
		filepath.Join(root, "client/templates/advanced/{{cookiecutter.project_slug}}/backtest.yaml"),
	}
	symbols := []string{"^NDX", "^GSPC", "^DJI", "^RUT", "SPY", "BRK.B", "AAPL", "FNMA"}
	for _, path := range files {
		symbols = append(symbols, templateSymbols(t, path)...)
	}
	seen := map[string]struct{}{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		symbol := strings.Trim(r.URL.Path, "/")
		if i := strings.IndexByte(symbol, '?'); i >= 0 {
			symbol = symbol[:i]
		}
		symbol, _ = urlPathUnescape(symbol)
		body := recordedYahooQuote(symbol, templateExchange(symbol))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	client := &YahooClient{crumb: "test", quoteURL: srv.URL + "/"}
	used := map[string]struct{}{}
	for _, symbol := range symbols {
		if _, ok := seen[strings.ToUpper(symbol)]; ok {
			continue
		}
		seen[strings.ToUpper(symbol)] = struct{}{}
		exchange, currency, err := client.Listing(context.Background(), symbol)
		require.NoError(t, err, symbol)
		require.Equal(t, "USD", currency, symbol)
		used[exchange] = struct{}{}
		_, refused, err := Classify("yahoo", symbol, Quote{Exchange: exchange, Currency: currency, Reported: true}, nil)
		require.NoError(t, err, symbol)
		require.False(t, refused, symbol)
	}
	for _, exchange := range []string{"NIM", "SNP", "DJI", "WCB", "PCX", "NYQ", "NMS", "PNK"} {
		_, ok := used[exchange]
		require.True(t, ok, exchange)
	}
}

func urlPathUnescape(symbol string) (string, error) {
	return strings.NewReplacer("%5E", "^", "%5e", "^").Replace(symbol), nil
}

func TestDefaultQuoteRoutesEachProvider(t *testing.T) {
	var yahooHits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		yahooHits++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(recordedYahooQuote("AAPL", "NMS")))
	}))
	t.Cleanup(srv.Close)
	client := &YahooClient{crumb: "test", quoteURL: srv.URL + "/"}
	quoteMu.Lock()
	prev := quoteYahoo
	quoteYahoo = client
	quoteCache = sync.Map{}
	quoteMu.Unlock()
	t.Cleanup(func() {
		quoteMu.Lock()
		quoteYahoo = prev
		quoteCache = sync.Map{}
		quoteMu.Unlock()
	})

	stooq, err := defaultQuote(context.Background(), "stooq", "", "AAPL")
	require.NoError(t, err)
	require.False(t, stooq.Reported)
	require.Zero(t, yahooHits)

	yahoo, err := defaultQuote(context.Background(), "yahoo", "", "AAPL")
	require.NoError(t, err)
	require.True(t, yahoo.Reported)
	require.Equal(t, "NMS", yahoo.Exchange)
	require.Equal(t, "USD", yahoo.Currency)
	require.Equal(t, 1, yahooHits)

	_, err = defaultQuote(context.Background(), "alpaca", "paper", "AAPL")
	require.Error(t, err)
	require.Equal(t, 1, yahooHits)
}
