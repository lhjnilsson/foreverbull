package marketdata

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// Yahoo requests use a client with a timeout, never http.DefaultClient.
func TestYahooDefaultClientHasTimeout(t *testing.T) {
	c := (&YahooClient{}).client()
	require.NotSame(t, http.DefaultClient, c)
	require.Equal(t, 30*time.Second, c.Timeout)
}

// A provider that never answers fails the request once the client times out.
func TestYahooRequestTimesOut(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}))
	defer srv.Close()
	defer close(release)

	y := &YahooClient{chartURL: srv.URL + "/", httpClient: &http.Client{Timeout: 50 * time.Millisecond}}
	started := time.Now()
	_, err := y.doRequest(srv.URL)
	require.Error(t, err)
	require.Less(t, time.Since(started), 5*time.Second)
}
