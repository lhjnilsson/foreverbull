package marketdata

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// stooqSessionServer answers like Stooq during a session, with yesterday's
// and today's rows whatever d2 says, and records each d2 asked for. Stooq's
// row for today carries prices while the market is open.
func stooqSessionServer(t *testing.T, now time.Time) (*StooqClient, *[]string) {
	t.Helper()
	asked := make([]string, 0)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asked = append(asked, r.URL.Query().Get("d2"))
		w.Header().Set("Content-Type", "text/csv")
		_, _ = w.Write([]byte("Date,Open,High,Low,Close,Volume\n" +
			"2026-10-29,10,10.5,9.5,10.2,100\n" +
			"2026-10-30,10.3,10.6,10.1,10.4,50\n"))
	}))
	t.Cleanup(srv.Close)
	client, err := NewStooqClient(WithStooqBaseURL(srv.URL))
	require.NoError(t, err)
	client.now = func() time.Time { return now }
	return client, &asked
}

func stooqDays(t *testing.T, client *StooqClient, start time.Time, end *time.Time) []string {
	t.Helper()
	bars, err := client.GetOHLC("AAPL", start, end)
	require.NoError(t, err)
	days := make([]string, 0, len(bars))
	for _, bar := range bars {
		days = append(days, bar.GetTimestamp().AsTime().UTC().Format(time.DateOnly))
	}
	return days
}

// Until 16:30 New York, when today's session counts as completed, Stooq's
// inclusive d2 stops at the previous session's date, and today's row is
// not returned to be stored even when Stooq sends it. That holds during the
// session and just after the 16:00 close.
func TestStooqGetOHLCLeavesTodaysUnfinishedSessionOut(t *testing.T) {
	t.Parallel()
	for _, at := range []string{"2026-10-30 11:00", "2026-10-30 16:05"} {
		client, asked := stooqSessionServer(t, newYork(t, at))
		start := time.Date(2026, 10, 29, 0, 0, 0, 0, time.UTC)
		today := time.Date(2026, 10, 30, 0, 0, 0, 0, time.UTC)
		tomorrow := today.AddDate(0, 0, 1)

		for _, end := range []*time.Time{&today, &tomorrow, nil} {
			require.Equal(t, []string{"2026-10-29"}, stooqDays(t, client, start, end), at)
		}
		require.Equal(t, []string{"20261029", "20261029", "20261029"}, *asked, at)
	}
}

// From 16:30 New York, today's session is completed and is fetched.
func TestStooqGetOHLCIncludesTodaysSessionAfterTheClose(t *testing.T) {
	t.Parallel()
	client, asked := stooqSessionServer(t, newYork(t, "2026-10-30 16:35"))
	start := time.Date(2026, 10, 29, 0, 0, 0, 0, time.UTC)
	tomorrow := time.Date(2026, 10, 31, 0, 0, 0, 0, time.UTC)

	require.Equal(t, []string{"2026-10-29", "2026-10-30"}, stooqDays(t, client, start, nil))
	require.Equal(t, []string{"2026-10-29", "2026-10-30"}, stooqDays(t, client, start, &tomorrow))
	require.Equal(t, []string{"20261030", "20261030"}, *asked)
}

// A fetch for today alone, before the close, asks Stooq for nothing.
func TestStooqGetOHLCForTodayOnlyBeforeTheCloseIsEmpty(t *testing.T) {
	t.Parallel()
	client, asked := stooqSessionServer(t, newYork(t, "2026-10-30 11:00"))

	require.Empty(t, stooqDays(t, client, time.Date(2026, 10, 30, 0, 0, 0, 0, time.UTC), nil))
	require.Empty(t, *asked)
}

func TestStooqLastCompletedDay(t *testing.T) {
	t.Parallel()
	today := time.Date(2026, 10, 30, 0, 0, 0, 0, time.UTC)
	yesterday := today.AddDate(0, 0, -1)
	past := today.AddDate(0, 0, -10)
	before := newYork(t, "2026-10-30 16:29")
	after := newYork(t, "2026-10-30 16:30")

	require.Equal(t, yesterday, stooqLastCompletedDay(today, before))
	require.Equal(t, yesterday, stooqLastCompletedDay(today.AddDate(0, 0, 5), before))
	require.Equal(t, today, stooqLastCompletedDay(today, after))
	require.Equal(t, today, stooqLastCompletedDay(today.AddDate(0, 0, 5), after))
	require.Equal(t, past, stooqLastCompletedDay(past, before))
}

// A fetch with no end asks Stooq up to today once today's session is
// completed, at 16:30 New York.
func TestStooqGetOHLCNilEndUsesTodayAfterTheClose(t *testing.T) {
	t.Parallel()
	var d2 string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		d2 = r.URL.Query().Get("d2")
		w.Header().Set("Content-Type", "text/csv")
		_, _ = w.Write([]byte("Date,Open,High,Low,Close,Volume\n2026-10-30,10,10.5,9.5,10.2,100\n"))
	}))
	t.Cleanup(srv.Close)
	client, err := NewStooqClient(WithStooqBaseURL(srv.URL))
	require.NoError(t, err)
	now := newYork(t, "2026-10-30 16:35")
	client.now = func() time.Time { return now }

	_, err = client.GetOHLC("AAPL", time.Date(2021, 1, 4, 0, 0, 0, 0, time.UTC), nil)
	require.NoError(t, err)
	require.Equal(t, "20261030", d2)
}
