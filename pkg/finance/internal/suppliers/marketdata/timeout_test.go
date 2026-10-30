package marketdata

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// The sentence is matched word for word by the backtest service.
func TestTimeoutErrorSentence(t *testing.T) {
	t.Parallel()
	err := &TimeoutError{Provider: "alpaca", Symbol: "AAPL"}
	require.Equal(t, "alpaca took too long to respond for AAPL", err.Error())
}

// The time-limit text is matched word for word by the backtest service and
// differs from the provider-timeout sentence.
func TestTimeLimitErrorText(t *testing.T) {
	t.Parallel()
	require.Equal(t, "the download hit its time limit of 2h0m0s at AAPL",
		(&TimeLimitError{Symbol: "AAPL", Limit: 2 * time.Hour}).Error())
}

func TestIsProviderTimeout(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	t.Cleanup(srv.Close)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL, nil)
	require.NoError(t, err)
	_, clientErr := (&http.Client{Timeout: 20 * time.Millisecond}).Do(req) //nolint:bodyclose // the request fails
	require.Error(t, clientErr)

	require.True(t, IsProviderTimeout(clientErr), "HTTP client timeout")
	require.True(t, IsProviderTimeout(fmt.Errorf("error getting bars: %w", context.DeadlineExceeded)), "fetch budget")
	require.False(t, IsProviderTimeout(context.Canceled), "a cancel is not a timeout")
	require.False(t, IsProviderTimeout(errors.New("HTTP 500")))
}
