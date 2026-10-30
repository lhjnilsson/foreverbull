package marketdata

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// indexTransport answers Yahoo's index and asset lookups and records how many
// asset lookups were in flight at once.
type indexTransport struct {
	components int

	mu          sync.Mutex
	inFlight    int
	maxInFlight int
	lookups     int
}

func (tr *indexTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	body := ""
	switch {
	case strings.Contains(req.URL.RawQuery, "components"):
		names := make([]string, tr.components)
		for i := range names {
			names[i] = fmt.Sprintf("%q", fmt.Sprintf("C%03d", i))
		}
		body = `{"quoteSummary":{"result":[{"components":{"components":[` + strings.Join(names, ",") + `]}}]}}`
	default:
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
		body = `{"quoteSummary":{"result":[{"quoteType":{"symbol":"` + symbol + `","quoteType":"INDEX","longName":"` + symbol + `"}}]}}`
	}
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
		Request:    req,
	}, nil
}

// GetIndex looks up each component with at most indexLookupWorkers requests
// in flight, not one goroutine per component, and returns every asset.
func TestGetIndexCapsConcurrentLookups(t *testing.T) {
	t.Parallel()
	tr := &indexTransport{components: 60}
	client := &YahooClient{
		crumb:      "test",
		httpClient: &http.Client{Transport: tr, Timeout: time.Minute},
		limiter:    newRequestLimiterEvery(time.Microsecond),
	}

	assets, err := client.GetIndex(context.Background(), "^OEX")
	require.NoError(t, err)
	require.Len(t, assets, 61, "every component and the index itself")
	require.Equal(t, "C000", assets[0].GetSymbol())
	require.Equal(t, "^OEX", assets[60].GetSymbol())
	require.Equal(t, 61, tr.lookups)
	require.LessOrEqual(t, tr.maxInFlight, indexLookupWorkers)
	require.Greater(t, tr.maxInFlight, 1, "lookups still run in parallel")
}
