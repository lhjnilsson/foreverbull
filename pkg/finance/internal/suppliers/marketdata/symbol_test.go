package marketdata

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/alpacahq/alpaca-trade-api-go/v3/marketdata"
	"github.com/stretchr/testify/require"
)

// Index holdings name Berkshire Hathaway class B as BRKB. Each provider
// knows it by its own ticker, and everything stored keeps the asked one.
func TestProviderSymbolMapsShareClasses(t *testing.T) {
	t.Parallel()
	cases := []struct {
		asked, yahoo, alpaca string
	}{
		{"BRKB", "BRK-B", "BRK.B"},
		{"brkb", "BRK-B", "BRK.B"},
		{"BRK.B", "BRK-B", "BRK.B"},
		{"BRK-B", "BRK-B", "BRK.B"},
		{"BRK/B", "BRK-B", "BRK.B"},
		{"BFB", "BF-B", "BF.B"},
		// Ordinary tickers, including ones that start like a share class.
		{"BK", "BK", "BK"},
		{"BKNG", "BKNG", "BKNG"},
		{"aapl", "AAPL", "AAPL"},
		{"GOOG", "GOOG", "GOOG"},
	}
	for _, tc := range cases {
		require.Equal(t, tc.yahoo, yahooSymbol(tc.asked), "yahoo %s", tc.asked)
		require.Equal(t, tc.alpaca, alpacaSymbol(tc.asked), "alpaca %s", tc.asked)
	}
}

func TestStooqResolvesShareClasses(t *testing.T) {
	t.Parallel()
	client, err := NewStooqClient()
	require.NoError(t, err)
	for asked, want := range map[string]string{"BRKB": "brk-b.us", "BRK.B": "brk-b.us", "BK": "bk.us"} {
		code, canonical, err := client.resolveSymbol(asked)
		require.NoError(t, err)
		require.Equal(t, want, code, asked)
		require.Equal(t, asked, canonical)
	}
	// An exchange suffix is still a Stooq code, not a share class.
	code, canonical, err := client.resolveSymbol("aapl.us")
	require.NoError(t, err)
	require.Equal(t, "aapl.us", code)
	require.Equal(t, "AAPL", canonical)
}

// recordingServer answers every request with body and records the paths.
func recordingServer(t *testing.T, body string) (*httptest.Server, func() []string) {
	t.Helper()
	var mu sync.Mutex
	paths := make([]string, 0)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		paths = append(paths, r.URL.Path+"?"+r.URL.RawQuery)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv, func() []string {
		mu.Lock()
		defer mu.Unlock()
		return append([]string(nil), paths...)
	}
}

const yahooBRKBChart = `{"chart":{"result":[{"meta":{"currency":"USD","symbol":"BRK-B"},
"timestamp":[1704205800],"indicators":{"quote":[{"open":[362.0],"high":[364.1],"low":[360.5],"close":[362.6],"volume":[3870000]}]}}]}}`

func TestYahooGetOHLCRequestsShareClassWithDash(t *testing.T) {
	t.Parallel()
	srv, paths := recordingServer(t, yahooBRKBChart)
	client := &YahooClient{crumb: "test", chartURL: srv.URL + "/"}
	start := time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC)
	end := time.Date(2024, 1, 3, 0, 0, 0, 0, time.UTC)

	bars, err := client.GetOHLC("BRKB", start, &end)
	require.NoError(t, err)
	require.Len(t, bars, 1)
	require.InDelta(t, 362.6, bars[0].Close, 1e-9)
	require.Len(t, paths(), 1)
	require.True(t, strings.HasPrefix(paths()[0], "/BRK-B?"), paths()[0])
}

func TestYahooGetAssetKeepsTheAskedShareClassTicker(t *testing.T) {
	t.Parallel()
	quote := `{"quoteSummary":{"result":[{"quoteType":{"exchange":"NYQ","symbol":"BRK-B",
"quoteType":"EQUITY","shortName":"Berkshire Hathaway Inc. New","longName":"Berkshire Hathaway Inc."}}]}}`
	srv, paths := recordingServer(t, quote)
	isin, _ := recordingServer(t, `{"stocks":[]}`)
	client := &YahooClient{crumb: "test", quoteURL: srv.URL + "/", isinURL: isin.URL + "/"}

	asset, err := client.GetAsset("BRKB")
	require.NoError(t, err)
	require.Equal(t, "BRKB", asset.Symbol, "stored and reported as the universe names it")
	require.Equal(t, "Berkshire Hathaway Inc.", asset.Name)
	require.True(t, strings.HasPrefix(paths()[0], "/BRK-B?"), paths()[0])
}

func TestAlpacaGetOHLCRequestsShareClassWithDot(t *testing.T) {
	t.Parallel()
	response, err := json.Marshal(map[string]any{"bars": map[string]any{"BRK.B": []map[string]any{
		{"t": "2024-01-02T05:00:00Z", "o": 362.0, "h": 364.1, "l": 360.5, "c": 362.6, "v": 3870000, "n": 1, "vw": 362.4},
	}}})
	require.NoError(t, err)
	srv, paths := recordingServer(t, string(response))
	client := marketdata.NewClient(marketdata.ClientOpts{
		APIKey: "key", APISecret: "secret", BaseURL: srv.URL, RetryLimit: 1,
	})
	bars := &AlpacaBars{md: client}
	start := time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC)
	end := time.Date(2024, 1, 3, 0, 0, 0, 0, time.UTC)

	ohlcs, err := bars.GetOHLC("BRKB", start, &end)
	require.NoError(t, err)
	require.Len(t, ohlcs, 1)
	require.InDelta(t, 362.6, ohlcs[0].Close, 1e-9)
	require.Len(t, paths(), 1)
	require.Contains(t, paths()[0], "BRK.B")
	require.NotContains(t, paths()[0], "BRKB")
}
