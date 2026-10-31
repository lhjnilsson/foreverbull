package marketdata

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/alpacahq/alpaca-trade-api-go/v3/marketdata"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// alpacaSessionServer answers like Alpaca during a session: daily bars
// stamped at midnight New York for yesterday and today, both priced, since
// Alpaca's bar for today has prices while the market is open. Like Alpaca
// it sends a bar only when its stamp is at or before the inclusive end, and
// it records every end asked for.
func alpacaSessionServer(t *testing.T, yesterday, today time.Time) (*AlpacaBars, *[]time.Time) {
	t.Helper()
	var mu sync.Mutex
	ends := make([]time.Time, 0)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		end, err := time.Parse(time.RFC3339Nano, r.URL.Query().Get("end"))
		if !assert.NoError(t, err) {
			http.Error(w, "bad end", http.StatusBadRequest)
			return
		}
		mu.Lock()
		ends = append(ends, end)
		mu.Unlock()
		body := `{"bars":{"XYZ":[{"t":"` + yesterday.UTC().Format(time.RFC3339) +
			`","o":10,"h":10.5,"l":9.5,"c":10.2,"v":100,"n":1,"vw":10.1}`
		if !today.After(end) {
			body += `,{"t":"` + today.UTC().Format(time.RFC3339) +
				`","o":10.3,"h":10.6,"l":10.1,"c":10.4,"v":50,"n":1,"vw":10.3}`
		}
		body += `]},"next_page_token":null}`
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	client := marketdata.NewClient(marketdata.ClientOpts{
		APIKey: "key", APISecret: "secret", BaseURL: srv.URL, RetryLimit: 1,
	})
	return &AlpacaBars{md: client}, &ends
}

func alpacaDays(t *testing.T, bars *AlpacaBars, start time.Time, end *time.Time) []string {
	t.Helper()
	ohlcs, err := bars.GetOHLC("XYZ", start, end)
	require.NoError(t, err)
	days := make([]string, 0, len(ohlcs))
	for _, ohlc := range ohlcs {
		days = append(days, ohlc.GetTimestamp().AsTime().In(xnysZone).Format(time.DateOnly))
	}
	return days
}

// Until 16:30 New York, when today's session counts as completed, a fetch
// that ends today or later stops just before midnight New York, so today's
// priced but unfinished bar is neither asked for nor returned to be
// stored. That holds during the session and just after the 16:00 close.
func TestAlpacaGetOHLCLeavesTodaysUnfinishedSessionOut(t *testing.T) {
	t.Parallel()
	for _, at := range []string{"2026-10-30 11:00", "2026-10-30 16:05"} {
		yesterday := newYork(t, "2026-10-29 00:00")
		today := newYork(t, "2026-10-30 00:00")
		bars, ends := alpacaSessionServer(t, yesterday, today)
		now := newYork(t, at)
		bars.now = func() time.Time { return now }
		start := time.Date(2026, 10, 29, 0, 0, 0, 0, time.UTC)
		tomorrow := time.Date(2026, 10, 31, 0, 0, 0, 0, time.UTC)

		require.Equal(t, []string{"2026-10-29"}, alpacaDays(t, bars, start, &tomorrow), at)
		require.Equal(t, []string{"2026-10-29"}, alpacaDays(t, bars, start, nil), at)
		require.Len(t, *ends, 2)
		for _, end := range *ends {
			require.True(t, end.Before(today), "at %s, end %s asks for today's bar", at, end)
		}
	}
}

// From 16:30 New York, today's session is completed and is fetched.
func TestAlpacaGetOHLCIncludesTodaysSessionAfterTheClose(t *testing.T) {
	t.Parallel()
	yesterday := newYork(t, "2026-10-29 00:00")
	today := newYork(t, "2026-10-30 00:00")
	bars, _ := alpacaSessionServer(t, yesterday, today)
	now := newYork(t, "2026-10-30 16:35")
	bars.now = func() time.Time { return now }
	start := time.Date(2026, 10, 29, 0, 0, 0, 0, time.UTC)
	tomorrow := time.Date(2026, 10, 31, 0, 0, 0, 0, time.UTC)

	require.Equal(t, []string{"2026-10-29", "2026-10-30"}, alpacaDays(t, bars, start, nil))
	require.Equal(t, []string{"2026-10-29", "2026-10-30"}, alpacaDays(t, bars, start, &tomorrow))
}

// A fetch for today alone, before the close, asks Alpaca for nothing.
func TestAlpacaGetOHLCForTodayOnlyBeforeTheCloseIsEmpty(t *testing.T) {
	t.Parallel()
	bars, ends := alpacaSessionServer(t, newYork(t, "2026-10-29 00:00"), newYork(t, "2026-10-30 00:00"))
	now := newYork(t, "2026-10-30 11:00")
	bars.now = func() time.Time { return now }

	require.Empty(t, alpacaDays(t, bars, newYork(t, "2026-10-30 00:00"), nil))
	require.Empty(t, *ends)
}

// A fetch that ends before today is asked for as given.
func TestAlpacaGetOHLCKeepsAPastEnd(t *testing.T) {
	t.Parallel()
	bars, ends := alpacaSessionServer(t, newYork(t, "2026-10-19 00:00"), newYork(t, "2026-10-20 00:00"))
	now := newYork(t, "2026-10-30 11:00")
	bars.now = func() time.Time { return now }
	end := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)

	require.Equal(t, []string{"2026-10-19", "2026-10-20"},
		alpacaDays(t, bars, time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC), &end))
	require.Equal(t, []time.Time{end}, *ends)
}

// Alpaca could send today's bar whatever the end asked for. Before 16:30
// New York it is unfinished and dropped.
func TestAlpacaGetOHLCDropsABarAfterTheLastCompletedSession(t *testing.T) {
	t.Parallel()
	yesterday := newYork(t, "2026-10-29 00:00")
	today := newYork(t, "2026-10-30 00:00")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"bars":{"XYZ":[{"t":"` + yesterday.UTC().Format(time.RFC3339) +
			`","o":10,"h":10.5,"l":9.5,"c":10.2,"v":100,"n":1,"vw":10.1},{"t":"` +
			today.UTC().Format(time.RFC3339) +
			`","o":10.3,"h":10.6,"l":10.1,"c":10.4,"v":50,"n":1,"vw":10.3}]},"next_page_token":null}`))
	}))
	t.Cleanup(srv.Close)
	bars := &AlpacaBars{md: marketdata.NewClient(marketdata.ClientOpts{
		APIKey: "key", APISecret: "secret", BaseURL: srv.URL, RetryLimit: 1,
	})}
	start := time.Date(2026, 10, 29, 0, 0, 0, 0, time.UTC)
	tomorrow := time.Date(2026, 10, 31, 0, 0, 0, 0, time.UTC)

	now := newYork(t, "2026-10-30 11:00")
	bars.now = func() time.Time { return now }
	require.Equal(t, []string{"2026-10-29"}, alpacaDays(t, bars, start, &tomorrow))
	require.Equal(t, []string{"2026-10-29"}, alpacaDays(t, bars, start, nil))

	// From 16:30 New York today's session is completed, and its bar is kept.
	now = newYork(t, "2026-10-30 16:35")
	require.Equal(t, []string{"2026-10-29", "2026-10-30"}, alpacaDays(t, bars, start, nil))
}
