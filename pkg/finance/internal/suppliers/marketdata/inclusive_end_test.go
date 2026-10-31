package marketdata

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// yahooRangeServer answers like Yahoo's chart API for daily bars: one
// priced bar per stamp given, each at the 09:30 New York open as Yahoo
// stamps them, sent when the stamp is at or after period1 and before the
// exclusive period2.
func yahooRangeServer(t *testing.T, stamps ...time.Time) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		period1, err := strconv.ParseInt(r.URL.Query().Get("period1"), 10, 64)
		if !assert.NoError(t, err) {
			http.Error(w, "bad period1", http.StatusBadRequest)
			return
		}
		period2, err := strconv.ParseInt(r.URL.Query().Get("period2"), 10, 64)
		if !assert.NoError(t, err) {
			http.Error(w, "bad period2", http.StatusBadRequest)
			return
		}
		sent := make([]string, 0, len(stamps))
		for _, stamp := range stamps {
			if stamp.Unix() >= period1 && stamp.Unix() < period2 {
				sent = append(sent, strconv.FormatInt(stamp.Unix(), 10))
			}
		}
		prices := func(value string) string {
			return "[" + strings.TrimSuffix(strings.Repeat(value+",", len(sent)), ",") + "]"
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"chart":{"result":[{"meta":{"currency":"USD","symbol":"XYZ"},"timestamp":[` +
			strings.Join(sent, ",") + `],"indicators":{"quote":[{"open":` + prices("10") + `,"high":` +
			prices("10.5") + `,"low":` + prices("9.5") + `,"close":` + prices("10.2") + `,"volume":` +
			prices("100") + `}]}}],"error":null}}`))
	}))
	t.Cleanup(srv.Close)
	return srv.URL + "/"
}

// stooqRangeServer answers like Stooq's daily CSV: one row per date given,
// sent when the date is from d1 through the inclusive d2.
func stooqRangeServer(t *testing.T, now time.Time, days ...string) *StooqClient {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		from, to := r.URL.Query().Get("d1"), r.URL.Query().Get("d2")
		body := "Date,Open,High,Low,Close,Volume\n"
		for _, day := range days {
			compact := strings.ReplaceAll(day, "-", "")
			if compact >= from && compact <= to {
				body += day + ",10,10.5,9.5,10.2,100\n"
			}
		}
		w.Header().Set("Content-Type", "text/csv")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	client, err := NewStooqClient(WithStooqBaseURL(srv.URL))
	require.NoError(t, err)
	client.now = func() time.Time { return now }
	return client
}

func yahooDays(t *testing.T, client *YahooClient, start time.Time, end *time.Time) []string {
	t.Helper()
	bars, err := client.GetOHLC("xyz", start, end)
	require.NoError(t, err)
	days := make([]string, 0, len(bars))
	for _, bar := range bars {
		days = append(days, bar.GetTimestamp().AsTime().In(xnysZone).Format(time.DateOnly))
	}
	return days
}

// stamps is each day at the given New York wall time.
func stamps(t *testing.T, clock string, days ...string) []time.Time {
	t.Helper()
	out := make([]time.Time, 0, len(days))
	for _, day := range days {
		out = append(out, newYork(t, day+" "+clock))
	}
	return out
}

// pastEndCase is a past end date, as a backtest sends it (midnight UTC),
// with the sessions around it. 2021-03-31 is in New York summer time
// (Alpaca stamps 04:00 UTC); 2021-01-29 is in winter time (05:00 UTC).
type pastEndCase struct {
	sessions []string
	start    time.Time
	end      time.Time
	want     []string
}

func pastEndCases() []pastEndCase {
	return []pastEndCase{
		{
			sessions: []string{"2021-03-29", "2021-03-30", "2021-03-31", "2021-04-01"},
			start:    time.Date(2021, 3, 29, 0, 0, 0, 0, time.UTC),
			end:      time.Date(2021, 3, 31, 0, 0, 0, 0, time.UTC),
			want:     []string{"2021-03-29", "2021-03-30", "2021-03-31"},
		},
		{
			sessions: []string{"2021-01-27", "2021-01-28", "2021-01-29", "2021-02-01"},
			start:    time.Date(2021, 1, 27, 0, 0, 0, 0, time.UTC),
			end:      time.Date(2021, 1, 29, 0, 0, 0, 0, time.UTC),
			want:     []string{"2021-01-27", "2021-01-28", "2021-01-29"},
		},
	}
}

// The end date is inclusive on Yahoo: period2 is the midnight New York
// after it, so the end date's bar, stamped at its 09:30 open, arrives. An
// end at 00:00 UTC on the date asked only for the days before it.
func TestYahooGetOHLCReturnsTheEndDatesBar(t *testing.T) {
	t.Parallel()
	now := newYork(t, "2026-10-30 11:00")
	for _, tc := range pastEndCases() {
		client := &YahooClient{
			chartURL: yahooRangeServer(t, stamps(t, "09:30", tc.sessions...)...),
			now:      func() time.Time { return now },
		}
		require.Equal(t, tc.want, yahooDays(t, client, tc.start, &tc.end))
	}
}

// The end date is inclusive on Alpaca: its daily bar is stamped at
// midnight New York, after 00:00 UTC on the same date, so the fetch ends
// just before the next midnight New York and the end date's bar arrives.
func TestAlpacaGetOHLCReturnsTheEndDatesBar(t *testing.T) {
	t.Parallel()
	now := newYork(t, "2026-10-30 11:00")
	for _, tc := range pastEndCases() {
		bars, ends := alpacaRangeServer(t, stamps(t, "00:00", tc.sessions...)...)
		bars.now = func() time.Time { return now }
		require.Equal(t, tc.want, alpacaDays(t, bars, tc.start, &tc.end))
		require.Len(t, *ends, 1)
		next := newYork(t, tc.want[len(tc.want)-1]+" 00:00").AddDate(0, 0, 1)
		require.True(t, (*ends)[0].Before(next), "end %s reaches the next day's bar", (*ends)[0])
	}
}

// All three providers return the same days, and so the same last date, for
// one past end, each with its own bar stamps and end semantics.
func TestProvidersReturnTheSameLastDateForAPastEnd(t *testing.T) {
	t.Parallel()
	now := newYork(t, "2026-10-30 11:00")
	for _, tc := range pastEndCases() {
		yahoo := &YahooClient{
			chartURL: yahooRangeServer(t, stamps(t, "09:30", tc.sessions...)...),
			now:      func() time.Time { return now },
		}
		alpaca, _ := alpacaRangeServer(t, stamps(t, "00:00", tc.sessions...)...)
		alpaca.now = func() time.Time { return now }
		stooq := stooqRangeServer(t, now, tc.sessions...)

		got := map[string][]string{
			"yahoo":  yahooDays(t, yahoo, tc.start, &tc.end),
			"alpaca": alpacaDays(t, alpaca, tc.start, &tc.end),
			"stooq":  stooqDays(t, stooq, tc.start, &tc.end),
		}
		for provider, days := range got {
			require.Equal(t, tc.want, days, provider)
		}
	}
}

// A provider that sends a bar after the end date anyway, as Yahoo's chart
// API can, has it dropped: the end date is the last one returned.
func TestYahooGetOHLCDropsABarAfterTheEndDate(t *testing.T) {
	t.Parallel()
	now := newYork(t, "2026-10-30 11:00")
	client := &YahooClient{
		chartURL: liveBarServer(t, newYork(t, "2021-03-31 09:30"), newYork(t, "2021-04-01 09:30"), "10.4"),
		now:      func() time.Time { return now },
	}
	end := time.Date(2021, 3, 31, 0, 0, 0, 0, time.UTC)
	require.Equal(t, []string{"2021-03-31"}, yahooDays(t, client, time.Date(2021, 3, 31, 0, 0, 0, 0, time.UTC), &end))
}

func TestInclusiveFetchEnd(t *testing.T) {
	t.Parallel()
	require.True(t, newYork(t, "2021-04-01 00:00").Equal(inclusiveFetchEnd(time.Date(2021, 3, 31, 0, 0, 0, 0, time.UTC))))
	require.True(t, newYork(t, "2021-01-30 00:00").Equal(inclusiveFetchEnd(time.Date(2021, 1, 29, 0, 0, 0, 0, time.UTC))))
}
