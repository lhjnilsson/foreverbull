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
	// failOn makes the lookup of this symbol answer with a body that
	// doesn't decode.
	failOn string

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
		if symbol == tr.failOn {
			body = "not json"
			break
		}
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

// The first failed lookup ends GetIndex with its error and no assets, and
// the lookups not yet sent are never sent.
func TestGetIndexStopsAtTheFirstError(t *testing.T) {
	t.Parallel()
	tr := &indexTransport{components: 60, failOn: "C002"}
	client := &YahooClient{
		crumb:      "test",
		httpClient: &http.Client{Transport: tr, Timeout: time.Minute},
		limiter:    newRequestLimiterEvery(time.Microsecond),
	}

	assets, err := client.GetIndex(context.Background(), "^OEX")
	require.ErrorContains(t, err, "error getting assets: error getting asset: error decoding response")
	require.Nil(t, assets)
	tr.mu.Lock()
	defer tr.mu.Unlock()
	require.Less(t, tr.lookups, 61, "the lookups after the error are not sent")
	require.LessOrEqual(t, tr.lookups, 3+indexLookupWorkers, "at most the workers in flight finish after C002")
}
