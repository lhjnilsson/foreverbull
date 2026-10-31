package marketdata

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// period2Server answers an empty chart and records the period2 asked for.
func period2Server(t *testing.T, period2 *int64) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		value, err := strconv.ParseInt(r.URL.Query().Get("period2"), 10, 64)
		if !assert.NoError(t, err) {
			http.Error(w, "bad period2", http.StatusBadRequest)
			return
		}
		*period2 = value
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"chart":{"result":[{"meta":{"currency":"USD","symbol":"XYZ"},` +
			`"timestamp":[],"indicators":{"quote":[{}]}}],"error":null}}`))
	}))
	t.Cleanup(srv.Close)
	return srv.URL + "/"
}

func newYork(t *testing.T, value string) time.Time {
	t.Helper()
	at, err := time.ParseInLocation("2006-01-02 15:04", value, xnysZone)
	require.NoError(t, err)
	return at
}

// Until 16:30 New York, when today's session counts as completed, a fetch
// that ends today or later stops at midnight New York, so today's
// unfinished session is not asked for and its null bar can't fail the
// download. That holds during the session and just after the 16:00 close.
func TestYahooGetOHLCLeavesTodaysUnfinishedSessionOut(t *testing.T) {
	t.Parallel()
	for _, at := range []string{"2026-10-30 11:00", "2026-10-30 16:05"} {
		var period2 int64
		now := newYork(t, at)
		client := &YahooClient{chartURL: period2Server(t, &period2), now: func() time.Time { return now }}
		start := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)

		tomorrow := time.Date(2026, 10, 31, 0, 0, 0, 0, time.UTC)
		_, err := client.GetOHLC("xyz", start, &tomorrow)
		require.NoError(t, err)
		require.Equal(t, newYork(t, "2026-10-30 00:00").Unix(), period2, at)

		_, err = client.GetOHLC("xyz", start, nil)
		require.NoError(t, err)
		require.Equal(t, newYork(t, "2026-10-30 00:00").Unix(), period2, at)
	}
}

// From 16:30 New York, today's session is completed and is fetched.
func TestYahooGetOHLCIncludesTodaysSessionAfterTheClose(t *testing.T) {
	t.Parallel()
	var period2 int64
	now := newYork(t, "2026-10-30 16:35")
	client := &YahooClient{chartURL: period2Server(t, &period2), now: func() time.Time { return now }}
	start := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)

	_, err := client.GetOHLC("xyz", start, nil)
	require.NoError(t, err)
	require.Equal(t, now.Unix(), period2)
}

// A fetch that ends before today asks for its whole end date: period2 is
// exclusive, so it is the midnight New York after it.
func TestYahooGetOHLCKeepsAPastEnd(t *testing.T) {
	t.Parallel()
	var period2 int64
	now := newYork(t, "2026-10-30 11:00")
	client := &YahooClient{chartURL: period2Server(t, &period2), now: func() time.Time { return now }}
	start := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC)

	_, err := client.GetOHLC("xyz", start, &end)
	require.NoError(t, err)
	require.Equal(t, newYork(t, "2026-10-21 00:00").Unix(), period2)
}

// The clamp itself, on a fixed clock, as the null-bar test sees it: a
// request that ends today before the close gets the previous completed
// session as its last bar, and a null on today's session never arrives.
func TestLastCompletedSessionEnd(t *testing.T) {
	t.Parallel()
	before := newYork(t, "2026-10-30 16:29")
	after := newYork(t, "2026-10-30 16:30")
	midnight := newYork(t, "2026-10-30 00:00")
	far := midnight.AddDate(0, 0, 7)

	require.Equal(t, midnight, lastCompletedSessionEnd(far, before))
	require.Equal(t, after, lastCompletedSessionEnd(far, after))
	past := midnight.AddDate(0, 0, -3)
	require.Equal(t, past, lastCompletedSessionEnd(past, before))
}

// todayServer answers like Yahoo during a session: yesterday's priced bar,
// and today's bar with a null close until the close. It only sends bars
// that open before period2.
func todayServer(t *testing.T, yesterday, today time.Time) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		period2, err := strconv.ParseInt(r.URL.Query().Get("period2"), 10, 64)
		if !assert.NoError(t, err) {
			http.Error(w, "bad period2", http.StatusBadRequest)
			return
		}
		body := `{"chart":{"result":[{"meta":{"currency":"USD","symbol":"XYZ"},"timestamp":[` +
			strconv.FormatInt(yesterday.Unix(), 10)
		quote := `"open":[10.0],"high":[10.5],"low":[9.5],"close":[10.2],"volume":[100]`
		if today.Unix() < period2 {
			body += "," + strconv.FormatInt(today.Unix(), 10)
			quote = `"open":[10.0,10.3],"high":[10.5,10.6],"low":[9.5,10.1],"close":[10.2,null],"volume":[100,50]`
		}
		body += `],"indicators":{"quote":[{` + quote + `}]}}],"error":null}}`
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv.URL + "/"
}

func TestYahooGetOHLCEndingTodayBeforeTheCloseDoesNotFail(t *testing.T) {
	t.Parallel()
	yesterday := newYork(t, "2026-10-29 09:30")
	today := newYork(t, "2026-10-30 09:30")
	now := newYork(t, "2026-10-30 11:00")
	client := &YahooClient{chartURL: todayServer(t, yesterday, today), now: func() time.Time { return now }}
	end := time.Date(2026, 10, 31, 0, 0, 0, 0, time.UTC)

	bars, err := client.GetOHLC("xyz", time.Date(2026, 10, 29, 0, 0, 0, 0, time.UTC), &end)
	require.NoError(t, err)
	require.Len(t, bars, 1)
	require.Equal(t, "2026-10-29", bars[0].GetTimestamp().AsTime().UTC().Format(time.DateOnly))

	// Without the clamp the same answer would carry today's null and fail.
	_, err = yahooBars([]int64{yesterday.Unix(), today.Unix()}, []yahooQuote{quoteWithNullAt(2, 1)})
	var missing *MissingPriceError
	require.ErrorAs(t, err, &missing)
	require.Equal(t, "2026-10-30", MissingPriceDays(missing.Days))
}

// A fetch for today alone, before the close, has nothing completed to ask
// for, so Yahoo isn't asked and there are no bars, not an error.
func TestYahooGetOHLCForTodayOnlyBeforeTheCloseIsEmpty(t *testing.T) {
	t.Parallel()
	var period2 int64
	now := newYork(t, "2026-10-30 11:00")
	client := &YahooClient{chartURL: period2Server(t, &period2), now: func() time.Time { return now }}

	bars, err := client.GetOHLC("xyz", newYork(t, "2026-10-30 00:00"), nil)
	require.NoError(t, err)
	require.Empty(t, bars)
	require.Zero(t, period2)
}

func TestCompletedFetchEnd(t *testing.T) {
	t.Parallel()
	now := newYork(t, "2026-10-30 11:00")
	midnight := newYork(t, "2026-10-30 00:00")
	far := midnight.AddDate(0, 0, 7)

	until, ok := completedFetchEnd(midnight.AddDate(0, 0, -1), far, now)
	require.True(t, ok)
	require.Equal(t, midnight, until)
	_, ok = completedFetchEnd(midnight, far, now)
	require.False(t, ok)
	_, ok = completedFetchEnd(midnight.Add(time.Hour), far, now)
	require.False(t, ok)
}

// liveBarServer answers like Yahoo's chart API at its worst: yesterday's
// priced bar and today's live bar whatever period2 says. Today's close is
// the given value, null while the session is open on a bad day.
func liveBarServer(t *testing.T, yesterday, today time.Time, todayClose string) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"chart":{"result":[{"meta":{"currency":"USD","symbol":"XYZ"},"timestamp":[` +
			strconv.FormatInt(yesterday.Unix(), 10) + "," + strconv.FormatInt(today.Unix(), 10) +
			`],"indicators":{"quote":[{"open":[10.0,10.3],"high":[10.5,10.6],"low":[9.5,10.1],` +
			`"close":[10.2,` + todayClose + `],"volume":[100,50]}]}}],"error":null}}`))
	}))
	t.Cleanup(srv.Close)
	return srv.URL + "/"
}

// Today's live bar sent past period2 is dropped before its prices are
// read, so a priced one is not stored as final and a null one does not
// fail the download.
func TestYahooGetOHLCDropsABarAfterTheLastCompletedSession(t *testing.T) {
	t.Parallel()
	yesterday := newYork(t, "2026-10-29 09:30")
	today := newYork(t, "2026-10-30 09:30")
	now := newYork(t, "2026-10-30 11:00")
	start := time.Date(2026, 10, 29, 0, 0, 0, 0, time.UTC)
	tomorrow := time.Date(2026, 10, 31, 0, 0, 0, 0, time.UTC)
	for _, todayClose := range []string{"10.4", "null"} {
		client := &YahooClient{
			chartURL: liveBarServer(t, yesterday, today, todayClose),
			now:      func() time.Time { return now },
		}
		for _, end := range []*time.Time{&tomorrow, nil} {
			bars, err := client.GetOHLC("xyz", start, end)
			require.NoError(t, err, todayClose)
			require.Len(t, bars, 1, todayClose)
			require.Equal(t, yesterday.Unix(), bars[0].GetTimestamp().AsTime().Unix(), todayClose)
		}
	}
}
