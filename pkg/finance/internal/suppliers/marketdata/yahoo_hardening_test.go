package marketdata

import (
	"context"
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
		isinSrv, isinURIs := uriServer(t, `{"stocks":[{"keywords":"`+symbol+`|US0000000001"}]}`)
		chartSrv, chartURIs := uriServer(t, `{"chart":{"result":[{"meta":{"currency":"USD","symbol":"X"},
"timestamp":[1704205800],"indicators":{"quote":[{"open":[1.0],"high":[1.0],"low":[1.0],"close":[1.0],"volume":[1]}]}}]}}`)
		client := &YahooClient{
			crumb: "test", quoteURL: quoteSrv.URL + "/", isinURL: isinSrv.URL + "/", chartURL: chartSrv.URL + "/",
		}

		_, err := client.GetAsset(context.Background(), symbol)
		require.NoError(t, err, symbol)
		require.Len(t, quoteURIs(), 1, symbol)
		require.True(t, strings.HasPrefix(quoteURIs()[0], "/"+escaped+"?crumb=test&modules=quoteType"), quoteURIs()[0])
		require.Equal(t, []string{"/" + escaped}, isinURIs(), symbol)

		start := time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC)
		end := time.Date(2024, 1, 3, 0, 0, 0, 0, time.UTC)
		_, err = client.GetOHLC(context.Background(), symbol, start, &end)
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
	_, err := client.GetOHLC(context.Background(), "^DJI", start, &end)
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
	_, err := client.getISIN(context.Background(), "AAPL")
	require.Error(t, err)
	var netErr net.Error
	require.True(t, errors.As(err, &netErr) && netErr.Timeout(), "want a timeout, got %v", err)
	require.Less(t, time.Since(began), 5*time.Second)
}

// An equity is matched across renames by its ISIN. A failed or empty ISIN
// lookup fails GetAsset instead of returning the equity without an ISIN.
// Other quote types have no ISIN lookup and still succeed.
func TestYahooGetAssetFailsWhenEquityISINLookupFails(t *testing.T) {
	t.Parallel()
	equity := `{"quoteSummary":{"result":[{"quoteType":{"exchange":"NMS","symbol":"META",
"quoteType":"EQUITY","shortName":"Meta","longName":"Meta Platforms, Inc."}}]}}`
	cases := []struct {
		name, isinBody string
		isinStatus     int
		wantErr        string
		// noISIN is true when the lookup answered that the equity has no
		// ISIN, as opposed to a failed request.
		noISIN bool
	}{
		{name: "isin service 503", isinStatus: http.StatusServiceUnavailable, isinBody: "Service Unavailable",
			wantErr: "error getting isin for symbol META: failed to get index data for symbol META: status 503 Service Unavailable"},
		{name: "isin not listed", isinStatus: http.StatusOK, isinBody: `{"stocks":[]}`,
			wantErr: "error getting isin for symbol META: ISIN not found for symbol META", noISIN: true},
		{name: "empty isin", isinStatus: http.StatusOK, isinBody: `{"stocks":[{"keywords":"META|"}]}`,
			wantErr: "error getting isin for symbol META: empty ISIN", noISIN: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			quoteSrv, _ := uriServer(t, equity)
			isinSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tc.isinStatus)
				_, _ = w.Write([]byte(tc.isinBody))
			}))
			t.Cleanup(isinSrv.Close)
			client := &YahooClient{crumb: "test", quoteURL: quoteSrv.URL + "/", isinURL: isinSrv.URL + "/"}

			asset, err := client.GetAsset(context.Background(), "META")
			require.Nil(t, asset)
			require.EqualError(t, err, tc.wantErr)
			if tc.noISIN {
				require.ErrorIs(t, err, ErrNoISIN)
			} else {
				require.NotErrorIs(t, err, ErrNoISIN)
			}
		})
	}

	t.Run("etf has no isin lookup", func(t *testing.T) {
		t.Parallel()
		quoteSrv, _ := uriServer(t, `{"quoteSummary":{"result":[{"quoteType":{"exchange":"PCX","symbol":"SPY",
"quoteType":"ETF","shortName":"SPDR","longName":"SPDR S&P 500 ETF Trust"}}]}}`)
		isinSrv, isinURIs := uriServer(t, `{"stocks":[]}`)
		client := &YahooClient{crumb: "test", quoteURL: quoteSrv.URL + "/", isinURL: isinSrv.URL + "/"}

		asset, err := client.GetAsset(context.Background(), "SPY")
		require.NoError(t, err)
		require.Equal(t, "SPY", asset.Symbol)
		require.Empty(t, asset.ISIN)
		require.Empty(t, isinURIs())
	})
}

// searchServers fakes Yahoo search (quotes), quoteSummary (every quote is
// an EQUITY) and the ISIN lookup. isinBody maps a symbol to its ISIN
// answer; a symbol missing from it gets HTTP 503.
func searchServers(t *testing.T, quotes []string, isinBody map[string]string) *YahooClient {
	t.Helper()
	search := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		parts := make([]string, 0, len(quotes))
		for _, q := range quotes {
			parts = append(parts, `{"symbol":"`+q+`"}`)
		}
		_, _ = w.Write([]byte(`{"quotes":[` + strings.Join(parts, ",") + `]}`))
	}))
	t.Cleanup(search.Close)
	quote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		symbol := strings.TrimPrefix(r.URL.Path, "/")
		_, _ = w.Write([]byte(`{"quoteSummary":{"result":[{"quoteType":{"symbol":"` + symbol +
			`","quoteType":"EQUITY","longName":"` + symbol + ` Inc."}}]}}`))
	}))
	t.Cleanup(quote.Close)
	isin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, ok := isinBody[strings.TrimPrefix(r.URL.Path, "/")]
		if !ok {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(isin.Close)
	return &YahooClient{crumb: "test", searchURL: search.URL + "/", quoteURL: quote.URL + "/", isinURL: isin.URL + "/"}
}

// A quote the ISIN lookup answers has no ISIN can't match the search by
// identity, so it is left out; the other quotes are returned.
func TestYahooSearchAssetSkipsQuoteWithNoISIN(t *testing.T) {
	t.Parallel()
	client := searchServers(t, []string{"META", "META.DE"}, map[string]string{
		"META":    `{"stocks":[{"keywords":"META|US30303M1027"}]}`,
		"META.DE": `{"stocks":[]}`,
	})

	assets, err := client.SearchAsset(context.Background(), "US30303M1027")
	require.NoError(t, err)
	require.Len(t, assets, 1)
	require.Equal(t, "META", assets[0].Symbol)
	require.Equal(t, "US30303M1027", assets[0].ISIN)
}

// Any other GetAsset failure, here a failed ISIN request, fails the whole
// search.
func TestYahooSearchAssetFailsWhenAQuoteLookupFails(t *testing.T) {
	t.Parallel()
	client := searchServers(t, []string{"META", "META.DE"}, map[string]string{
		"META": `{"stocks":[{"keywords":"META|US30303M1027"}]}`,
	})

	assets, err := client.SearchAsset(context.Background(), "US30303M1027")
	require.Nil(t, assets)
	require.EqualError(t, err, "error getting asset: error getting isin for symbol META.DE: "+
		"failed to get index data for symbol META.DE: status 503 Service Unavailable")
	require.NotErrorIs(t, err, ErrNoISIN)
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
	assets, err := client.SearchAsset(context.Background(), q)
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
	for _, tc := range []struct{ raw, crumb, param, want string }{
		{"https://y.example/q/AAPL", "", "modules=quoteType", "https://y.example/q/AAPL?modules=quoteType"},
		{"https://y.example/q/AAPL", "ab/c=", "modules=quoteType", "https://y.example/q/AAPL?crumb=ab%2Fc%3D&modules=quoteType"},
		{"https://y.example/s?q=A%26B&region=US", "x", "", "https://y.example/s?q=A%26B&region=US&crumb=x"},
	} {
		var params []string
		if tc.param != "" {
			params = append(params, tc.param)
		}
		got, err := requestURL(tc.raw, tc.crumb, params...)
		require.NoError(t, err)
		require.Equal(t, tc.want, got)
	}
}

// An unparsable URL fails the request instead of being sent as is, and the
// error names neither the URL nor any of its query values.
func TestYahooRequestURLRefusesAnUnparsableURL(t *testing.T) {
	t.Parallel()
	got, err := requestURL("https://y.example/q/\x7fAAPL?crumb=s3cr3tcrumb", "s3cr3tcrumb", "modules=quoteType")
	require.Empty(t, got)
	require.EqualError(t, err, "invalid request URL: net/url: invalid control character in URL")

	client := &YahooClient{crumb: "s3cr3tcrumb", quoteURL: "https://y.example/q/\x7f?crumb=s3cr3tcrumb&"}
	asset, err := client.GetAsset(context.Background(), "AAPL")
	require.Nil(t, asset)
	require.EqualError(t, err, "error: invalid request URL: net/url: invalid control character in URL")
	for _, leak := range []string{"s3cr3tcrumb", "y.example", "crumb"} {
		require.NotContains(t, err.Error(), leak)
	}
}
