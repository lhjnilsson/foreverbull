package marketdata

import (
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// uriServer answers every request with body and records the raw request
// URI, so an escaped `/` (%2F) is not decoded away as it is in URL.Path.
func uriServer(t *testing.T, body string) (*httptest.Server, func() []string) {
	t.Helper()
	var mu sync.Mutex
	uris := make([]string, 0)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		uris = append(uris, r.RequestURI)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv, func() []string {
		mu.Lock()
		defer mu.Unlock()
		return append([]string(nil), uris...)
	}
}

// A symbol is one path segment. A `/` or `?` in it must not add a path
// segment or start a query on the Yahoo quote, Yahoo chart or ISIN URL.
func TestYahooEscapesSymbolInEveryURL(t *testing.T) {
	t.Parallel()
	for symbol, escaped := range map[string]string{
		"AB/CD?X=1": "AB%2FCD%3FX=1",
		"Q?Z":       "Q%3FZ",
		"A/B/C":     "A%2FB%2FC",
	} {
		quote := `{"quoteSummary":{"result":[{"quoteType":{"exchange":"NYQ","symbol":"` + symbol +
			`","quoteType":"EQUITY","shortName":"Test","longName":"Test Inc."}}]}}`
		quoteSrv, quoteURIs := uriServer(t, quote)
		isinSrv, isinURIs := uriServer(t, `{"stocks":[]}`)
		chartSrv, chartURIs := uriServer(t, `{"chart":{"result":[{"meta":{"currency":"USD","symbol":"X"},
"timestamp":[1704205800],"indicators":{"quote":[{"open":[1.0],"high":[1.0],"low":[1.0],"close":[1.0],"volume":[1]}]}}]}}`)
		client := &YahooClient{
			crumb: "test", quoteURL: quoteSrv.URL + "/", isinURL: isinSrv.URL + "/", chartURL: chartSrv.URL + "/",
		}

		_, err := client.GetAsset(symbol)
		require.NoError(t, err, symbol)
		require.Len(t, quoteURIs(), 1, symbol)
		require.True(t, strings.HasPrefix(quoteURIs()[0], "/"+escaped+"?crumb=test&modules=quoteType"), quoteURIs()[0])
		require.Equal(t, []string{"/" + escaped}, isinURIs(), symbol)

		start := time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC)
		end := time.Date(2024, 1, 3, 0, 0, 0, 0, time.UTC)
		_, err = client.GetOHLC(symbol, start, &end)
		require.NoError(t, err, symbol)
		require.Len(t, chartURIs(), 1, symbol)
		require.True(t, strings.HasPrefix(chartURIs()[0], "/"+escaped+"?crumb=test&period1="), chartURIs()[0])
	}
}

// An index symbol keeps its caret, escaped, on the chart request.
func TestYahooChartEscapesIndexCaret(t *testing.T) {
	t.Parallel()
	chartSrv, chartURIs := uriServer(t, `{"chart":{"result":[{"meta":{"currency":"USD","symbol":"^DJI"},
"timestamp":[1704205800],"indicators":{"quote":[{"open":[1.0],"high":[1.0],"low":[1.0],"close":[1.0],"volume":[1]}]}}]}}`)
	client := &YahooClient{crumb: "test", chartURL: chartSrv.URL + "/"}

	start := time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC)
	end := time.Date(2024, 1, 3, 0, 0, 0, 0, time.UTC)
	_, err := client.GetOHLC("^DJI", start, &end)
	require.NoError(t, err)
	require.Len(t, chartURIs(), 1)
	require.True(t, strings.HasPrefix(chartURIs()[0], "/%5EDJI?crumb=test&period1="), chartURIs()[0])
}

// getISIN uses the client's timeout; a server that never answers fails the
// lookup instead of hanging the download.
func TestGetISINTimesOutOnSlowServer(t *testing.T) {
	t.Parallel()
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}))
	t.Cleanup(func() {
		close(release)
		srv.Close()
	})
	client := &YahooClient{isinURL: srv.URL + "/", httpClient: &http.Client{Timeout: 50 * time.Millisecond}}

	began := time.Now()
	_, err := client.getISIN("AAPL")
	require.Error(t, err)
	var netErr net.Error
	require.True(t, errors.As(err, &netErr) && netErr.Timeout(), "want a timeout, got %v", err)
	require.Less(t, time.Since(began), 5*time.Second)
}

func TestYahooClientDefaultsToSharedClientWithTimeout(t *testing.T) {
	t.Parallel()
	client := &YahooClient{}
	require.Same(t, yahooHTTPClient, client.client())
	require.Equal(t, yahooHTTPTimeout, client.client().Timeout)
	require.Positive(t, client.client().Timeout)
}

// A search query is one q value: `&`, `?`, `#` and a space in it must not add
// parameters or cut the URL short.
func TestYahooSearchEscapesQuery(t *testing.T) {
	t.Parallel()
	var mu sync.Mutex
	var queries []map[string][]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		queries = append(queries, r.URL.Query())
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"quotes":[]}`))
	}))
	t.Cleanup(srv.Close)
	client := &YahooClient{searchURL: srv.URL + "/v1/finance/search", crumb: "ab/c="}

	q := "A&B=1?C#D E"
	assets, err := client.SearchAsset(q)
	require.NoError(t, err)
	require.Empty(t, assets)
	mu.Lock()
	defer mu.Unlock()
	require.Len(t, queries, 1)
	require.Equal(t, []string{q}, queries[0]["q"])
	require.NotContains(t, queries[0], "B")
	require.Equal(t, []string{"US"}, queries[0]["region"])
	require.Equal(t, []string{"ab/c="}, queries[0]["crumb"], "the crumb is its own parameter")
}

func TestYahooRequestURLKeepsEveryParameter(t *testing.T) {
	t.Parallel()
	require.Equal(t, "https://y.example/q/AAPL?modules=quoteType",
		requestURL("https://y.example/q/AAPL", "", "modules=quoteType"))
	require.Equal(t, "https://y.example/q/AAPL?crumb=ab%2Fc%3D&modules=quoteType",
		requestURL("https://y.example/q/AAPL", "ab/c=", "modules=quoteType"))
	require.Equal(t, "https://y.example/s?q=A%26B&region=US&crumb=x",
		requestURL("https://y.example/s?q=A%26B&region=US", "x"))
}
