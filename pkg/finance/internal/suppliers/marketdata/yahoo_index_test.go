package marketdata

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// indexServer fakes Yahoo's quoteSummary for an index and its members. Each
// answer waits a random 0-2ms so member lookups finish out of order. A
// member in fail answers with a quoteSummary error.
func indexServer(t *testing.T, indexBody string, fail map[string]bool) (*YahooClient, func() []string) {
	t.Helper()
	var mu sync.Mutex
	var paths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		paths = append(paths, r.URL.EscapedPath())
		mu.Unlock()
		time.Sleep(time.Duration(rand.IntN(2000)) * time.Microsecond) //nolint:gosec // test jitter
		symbol, err := url.PathUnescape(strings.TrimPrefix(r.URL.EscapedPath(), "/"))
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(r.URL.Query().Get("modules"), "components"):
			_, _ = w.Write([]byte(indexBody))
		case fail[symbol]:
			_, _ = fmt.Fprintf(w, `{"quoteSummary":{"result":null,"error":{"code":"Internal","description":"lookup of %s broke"}}}`, symbol)
		default:
			_, _ = fmt.Fprintf(w, `{"quoteSummary":{"result":[{"quoteType":{"symbol":%q,"quoteType":"ETF","longName":%q}}]}}`,
				symbol, symbol+" Fund")
		}
	}))
	t.Cleanup(srv.Close)
	return &YahooClient{crumb: "test", quoteURL: srv.URL + "/"}, func() []string {
		mu.Lock()
		defer mu.Unlock()
		return append([]string(nil), paths...)
	}
}

func indexBody(t *testing.T, members []string) string {
	t.Helper()
	body, err := json.Marshal(map[string]any{"quoteSummary": map[string]any{"result": []any{
		map[string]any{"components": map[string]any{"components": members}},
	}}})
	require.NoError(t, err)
	return string(body)
}

func indexMembers(n int) []string {
	members := make([]string, 0, n)
	for i := range n {
		members = append(members, fmt.Sprintf("M%03d", i))
	}
	return members
}

// Every member lookup is in before GetIndex returns. Run with -race and
// -count=20: the collector used to race with the return.
func TestYahooGetIndexReturnsEveryMember(t *testing.T) {
	t.Parallel()
	members := indexMembers(120)
	client, _ := indexServer(t, indexBody(t, members), nil)

	assets, err := client.GetIndex(context.Background(), "^OEF")
	require.NoError(t, err)
	require.Len(t, assets, len(members)+1, "every member plus the index asset")
	for i, member := range members {
		require.Equal(t, member, assets[i].Symbol)
	}
	require.Equal(t, "^OEF", assets[len(members)].Symbol)
}

// One failing member fails the index, and the error names it.
func TestYahooGetIndexFailsAndNamesTheFailingMember(t *testing.T) {
	t.Parallel()
	members := indexMembers(30)
	client, _ := indexServer(t, indexBody(t, members), map[string]bool{"M007": true})

	assets, err := client.GetIndex(context.Background(), "^OEF")
	require.Nil(t, assets)
	require.EqualError(t, err, "error getting assets: component M007: lookup of M007 broke")
}

// Every failing member is named, not only the first.
func TestYahooGetIndexNamesEveryFailingMember(t *testing.T) {
	t.Parallel()
	members := indexMembers(30)
	client, _ := indexServer(t, indexBody(t, members), map[string]bool{"M003": true, "M021": true})

	assets, err := client.GetIndex(context.Background(), "^OEF")
	require.Nil(t, assets)
	require.EqualError(t, err,
		"error getting assets: component M003: lookup of M003 broke\ncomponent M021: lookup of M021 broke")
}

// An index answer with no result is an error, not a panic.
func TestYahooGetIndexEmptyResultIsAnError(t *testing.T) {
	t.Parallel()
	client, paths := indexServer(t, `{"quoteSummary":{"result":[]}}`, nil)

	require.NotPanics(t, func() {
		got, err := client.GetIndex(context.Background(), "^OEF")
		require.Nil(t, got)
		require.EqualError(t, err, "fail to get index data for symbol ^OEF: no result")
	})
	require.Len(t, paths(), 1, "no member is looked up")
}

// The index symbol is one escaped path segment on the configured quote
// URL: a `/` or `?` in it can't add a path segment or start a query.
func TestYahooGetIndexEscapesSymbolInPath(t *testing.T) {
	t.Parallel()
	for symbol, escaped := range map[string]string{
		"^AB/CD?X=1": "/%5EAB%2FCD%3FX=1",
		"Q?Z":        "/Q%3FZ",
		"A/B/C":      "/A%2FB%2FC",
	} {
		client, paths := indexServer(t, indexBody(t, nil), nil)
		_, err := client.GetIndex(context.Background(), symbol)
		require.NoError(t, err, symbol)
		require.NotEmpty(t, paths(), symbol)
		require.Equal(t, escaped, paths()[0], symbol)
	}
}

// countingIndexTransport answers Yahoo's index and asset lookups and records
// how many asset lookups were in flight at once.
type countingIndexTransport struct {
	components int

	mu          sync.Mutex
	inFlight    int
	maxInFlight int
	lookups     int
}

func (tr *countingIndexTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	body := ""
	if strings.Contains(req.URL.RawQuery, "components") {
		names := make([]string, tr.components)
		for i := range names {
			names[i] = fmt.Sprintf("%q", fmt.Sprintf("C%03d", i))
		}
		body = `{"quoteSummary":{"result":[{"components":{"components":[` + strings.Join(names, ",") + `]}}]}}`
	} else {
		tr.mu.Lock()
		tr.inFlight++
		tr.lookups++
		tr.maxInFlight = max(tr.maxInFlight, tr.inFlight)
		tr.mu.Unlock()
		time.Sleep(5 * time.Millisecond)
		tr.mu.Lock()
		tr.inFlight--
		tr.mu.Unlock()
		symbol := req.URL.Path[strings.LastIndex(req.URL.Path, "/")+1:]
		// Not EQUITY, so no ISIN lookup on another site.
		body = `{"quoteSummary":{"result":[{"quoteType":{"symbol":"` + symbol +
			`","quoteType":"INDEX","longName":"` + symbol + `"}}]}}`
	}
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
		Request:    req,
	}, nil
}

// GetIndex looks up each member with at most indexLookupWorkers requests in
// flight, not one goroutine per member, and still returns every asset.
func TestYahooGetIndexCapsConcurrentLookups(t *testing.T) {
	t.Parallel()
	tr := &countingIndexTransport{components: 60}
	client := &YahooClient{
		crumb:      "test",
		httpClient: &http.Client{Transport: tr, Timeout: time.Minute},
		limiter:    newRequestLimiterEvery(time.Microsecond),
	}

	assets, err := client.GetIndex(context.Background(), "^OEX")
	require.NoError(t, err)
	require.Len(t, assets, 61, "every member and the index itself")
	require.Equal(t, "C000", assets[0].GetSymbol())
	require.Equal(t, "^OEX", assets[60].GetSymbol())
	require.Equal(t, 61, tr.lookups)
	require.LessOrEqual(t, tr.maxInFlight, indexLookupWorkers)
	require.Greater(t, tr.maxInFlight, 1, "lookups still run in parallel")
}
