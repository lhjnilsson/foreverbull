package marketdata

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/alpacahq/alpaca-trade-api-go/v3/marketdata"
	"github.com/lhjnilsson/foreverbull/pkg/finance/internal/mapping"
	"github.com/stretchr/testify/require"
)

// Index holdings name Berkshire Hathaway class B as BRKB. Each provider
// knows it by its own ticker, and everything stored keeps the asked one.
func TestProviderSymbolMapsShareClasses(t *testing.T) {
	t.Parallel()
	cases := []struct {
		asked, yahoo, alpaca string
	}{
		{"BRK.B", "BRK-B", "BRK.B"},
		{"BRK-B", "BRK-B", "BRK.B"},
		{"BRK/B", "BRK-B", "BRK.B"},
		{"BF.B", "BF-B", "BF.B"},
		// Ordinary tickers, including ones that start like a share class.
		{"BK", "BK", "BK"},
		{"BKNG", "BKNG", "BKNG"},
		{"aapl", "AAPL", "AAPL"},
		{"GOOG", "GOOG", "GOOG"},
		// An exchange suffix is sent as written. A joined iShares ticker is
		// not guessed from the letters.
		{"VOD.L", "VOD.L", "VOD.L"},
		{"BRKB", "BRKB", "BRKB"},
		{"BFB", "BFB", "BFB"},
	}
	for _, tc := range cases {
		require.Equal(t, tc.yahoo, yahooSymbol(tc.asked), "yahoo %s", tc.asked)
		require.Equal(t, tc.alpaca, alpacaSymbol(tc.asked), "alpaca %s", tc.asked)
	}
}

func TestStooqResolvesShareClasses(t *testing.T) {
	t.Parallel()
	client, err := NewStooqClient(WithStooqExchange("US"))
	require.NoError(t, err)
	classes := mapping.NewClasses()
	classes.Learn("BRK-B")
	client.classes = classes
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
	classes := mapping.NewClasses()
	classes.Learn("BRK-B")
	client := &YahooClient{crumb: "test", chartURL: srv.URL + "/", classes: classes}
	start := time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC)
	end := time.Date(2024, 1, 3, 0, 0, 0, 0, time.UTC)

	bars, err := client.GetOHLC(context.Background(), "BRKB", start, &end)
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
	isin, _ := recordingServer(t, `{"stocks":[{"keywords":"BRK-B|US0846707026"}]}`)
	classes := mapping.NewClasses()
	classes.Learn("BRK-B")
	client := &YahooClient{crumb: "test", quoteURL: srv.URL + "/", isinURL: isin.URL + "/", classes: classes}

	asset, err := client.GetAsset(context.Background(), "BRKB")
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
	classes := mapping.NewClasses()
	classes.Learn("BRK-B")
	bars := &AlpacaBars{opts: marketdata.ClientOpts{
		APIKey: "key", APISecret: "secret", BaseURL: srv.URL,
	}, classes: classes}
	start := time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC)
	end := time.Date(2024, 1, 3, 0, 0, 0, 0, time.UTC)

	ohlcs, err := bars.GetOHLC(context.Background(), "BRKB", start, &end)
	require.NoError(t, err)
	require.Len(t, ohlcs, 1)
	require.InDelta(t, 362.6, ohlcs[0].Close, 1e-9)
	require.Len(t, paths(), 1)
	require.Contains(t, paths()[0], "BRK.B")
	require.NotContains(t, paths()[0], "BRKB")
}

// VOD.L is Vodafone on London, not class L: every provider is asked for it as written.
func TestExchangeSuffixIsNotAShareClass(t *testing.T) {
	t.Parallel()
	require.False(t, parseShareClass("VOD.L"))
	_, ok := stooqShareClassCode("VOD.L", "us")
	require.False(t, ok)

	srv, paths := recordingServer(t, `{"chart":{"result":[],"error":null}}`)
	client := &YahooClient{crumb: "test", chartURL: srv.URL + "/"}
	start := time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC)
	end := time.Date(2024, 1, 3, 0, 0, 0, 0, time.UTC)
	_, _ = client.GetOHLC(context.Background(), "VOD.L", start, &end)
	require.NotEmpty(t, paths())
	require.True(t, strings.HasPrefix(paths()[0], "/VOD.L?"), paths()[0])

	response, err := json.Marshal(map[string]any{"bars": map[string]any{"VOD.L": []map[string]any{
		{"t": "2024-01-02T05:00:00Z", "o": 70.0, "h": 71.0, "l": 69.5, "c": 70.4, "v": 1000, "n": 1, "vw": 70.2},
	}}})
	require.NoError(t, err)
	alpacaSrv, alpacaPaths := recordingServer(t, string(response))
	bars := &AlpacaBars{opts: marketdata.ClientOpts{
		APIKey: "key", APISecret: "secret", BaseURL: alpacaSrv.URL,
	}}
	_, err = bars.GetOHLC(context.Background(), "VOD.L", start, &end)
	require.NoError(t, err)
	require.Len(t, alpacaPaths(), 1)
	require.Contains(t, alpacaPaths()[0], "VOD.L")
	require.NotContains(t, alpacaPaths()[0], "VOD-L")
}

// Every spelling of a listed share class stores under one name. Any other
// one-letter suffix (an exchange such as VOD.L) is kept as typed.
func TestStoredSymbol(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ asked, want string }{
		{"BRK.B", "BRKB"}, {"BRK-B", "BRKB"}, {"BRKB", "BRKB"}, {"BRK/B", "BRKB"},
		{"BRK B", "BRKB"}, {"brk.b", "BRKB"},
		{"BRK.A", "BRKA"}, {"BRK-A", "BRKA"}, {"BRK/A", "BRKA"}, {"BRKA", "BRKA"},
		{"BF.B", "BFB"},
		// Exchange suffixes and unlisted classes: unchanged, as typed.
		{"VOD.L", "VOD.L"}, {"vod.l", "vod.l"}, {"SHOP.V", "SHOP.V"}, {"RDS.A", "RDSA"}, {"BRK.C", "BRK.C"},
		// Not share classes: unchanged.
		{"BK", "BK"}, {"BKNG", "BKNG"}, {"AAPL", "AAPL"}, {"aapl.us", "aapl.us"}, {"SPY", "SPY"}, {"", ""},
	} {
		require.Equal(t, tc.want, StoredSymbol(tc.asked), tc.asked)
	}
}

// A joined iShares ticker is not a share class until a listing symbol for
// that ISIN says so. The request then uses the provider's spelling, so
// Yahoo is not asked for BRKB.
func TestJoinedTickerNeedsAListingSymbol(t *testing.T) {
	t.Parallel()
	require.Equal(t, "BRKB", yahooSymbol("BRKB"))
	classes := mapping.NewClasses()
	classes.LearnIfJoined("BRKB", []string{"BRK-B"})
	require.Equal(t, "BRK-B", mapping.FormatYahoo(classes, "BRKB"))
	require.Equal(t, "BRKB", mapping.Stored("BRK.B"))
	require.Equal(t, "BRKB", mapping.Stored("BRKB"))
}
