package marketdata

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func yahooFixtureServer(t *testing.T, path string) *YahooClient {
	t.Helper()
	body, err := os.ReadFile(path)
	require.NoError(t, err)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return &YahooClient{chartURL: srv.URL + "/"}
}

// A null in Yahoo's price arrays is a missing session. It must not decode
// to a 0 price, which reads as a -100% day in every metric.
func TestYahooGetOHLCLeavesNullBarsOut(t *testing.T) {
	t.Parallel()
	client := yahooFixtureServer(t, "testdata/yahoo_chart_null_bars.json")
	start := time.Date(2026, 5, 4, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 5, 9, 0, 0, 0, 0, time.UTC)

	bars, err := client.GetOHLC("xyz", start, &end)
	require.NoError(t, err)

	days := make([]string, 0, len(bars))
	for _, bar := range bars {
		days = append(days, bar.Timestamp.AsTime().UTC().Format("2006-01-02"))
		require.Positive(t, bar.Open)
		require.Positive(t, bar.High)
		require.Positive(t, bar.Low)
		require.Positive(t, bar.Close)
	}
	// 2026-05-05 is all null; 2026-05-07 has a null open only.
	require.Equal(t, []string{"2026-05-04", "2026-05-06", "2026-05-08"}, days)
	require.InDelta(t, 10.7, bars[1].Close, 1e-9)
	// A null volume on a priced session is no shares traded, not a missing bar.
	require.Equal(t, uint64(0), bars[2].Volume)
}

func TestYahooBarsReportsMissingSessions(t *testing.T) {
	t.Parallel()
	price := func(v float64) *float64 { return &v }
	volume := uint64(5)
	quote := yahooQuote{
		Open:   []*float64{price(1), nil, price(0), price(1)},
		High:   []*float64{price(1), nil, price(1), price(1)},
		Low:    []*float64{price(1), nil, price(1), price(-1)},
		Close:  []*float64{price(1), nil, price(1), price(1)},
		Volume: []*uint64{&volume, nil, &volume, &volume},
	}
	timestamps := []int64{1777901400, 1777987800, 1778074200, 1778160600}

	bars, missing, err := yahooBars(timestamps, []yahooQuote{quote})
	require.NoError(t, err)
	require.Len(t, bars, 1)
	require.Len(t, missing, 3, "null, zero, and negative prices are all missing bars")
	require.Equal(t, "2026-05-05", missing[0].UTC().Format("2006-01-02"))
}

func TestYahooBarsRefusesColumnsOfDifferentLength(t *testing.T) {
	t.Parallel()
	price := 1.0
	quote := yahooQuote{
		Open:   []*float64{&price, &price},
		High:   []*float64{&price, &price},
		Low:    []*float64{&price, &price},
		Close:  []*float64{&price},
		Volume: []*uint64{nil, nil},
	}
	_, _, err := yahooBars([]int64{1777901400, 1777987800}, []yahooQuote{quote})
	require.EqualError(t, err, "close has 1 values for 2 timestamps")
}

func yahooChartServer(t *testing.T, status int, contentType, body string) *YahooClient {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", contentType)
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return &YahooClient{chartURL: srv.URL + "/"}
}

// Yahoo's "Not Found" chart error is its answer that a ticker has no bars,
// so a download may try another ticker or report no prices. A server error
// is a failed request and must not read as no bars.
func TestYahooGetOHLCNotFoundIsNoBars(t *testing.T) {
	t.Parallel()
	start := time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2021, 2, 1, 0, 0, 0, 0, time.UTC)

	notFound := yahooChartServer(t, http.StatusNotFound, "application/json",
		`{"chart":{"result":null,"error":{"code":"Not Found","description":"No data found, symbol may be delisted"}}}`)
	bars, err := notFound.GetOHLC("NON_EXISTING", start, &end)
	require.Nil(t, bars)
	require.ErrorIs(t, err, ErrNoBars)
	require.EqualError(t, err, "fail to get OHLC data for symbol NON_EXISTING: No data found, symbol may be delisted")

	// 404 with the Not Found chart error is the same answer.
	notFound404 := yahooChartServer(t, http.StatusNotFound, "application/json",
		`{"chart":{"result":null,"error":{"code":"Not Found","description":"No data found, symbol may be delisted"}}}`)
	_, err = notFound404.GetOHLC("NON_EXISTING", start, &end)
	require.ErrorIs(t, err, ErrNoBars)

	cases := []struct {
		name, contentType, body string
		status                  int
	}{
		{name: "chart error", status: http.StatusInternalServerError, contentType: "application/json",
			body: `{"chart":{"result":null,"error":{"code":"Internal Server Error","description":"try again"}}}`},
		{name: "plain 503", status: http.StatusServiceUnavailable, contentType: "text/plain", body: "Service Unavailable"},
		// A 5xx with an empty-looking chart must not read as 0 bars.
		{name: "503 with empty result", status: http.StatusServiceUnavailable, contentType: "application/json",
			body: `{"chart":{"result":[{}]}}`},
		// Not Found counts only on a 200 or 404.
		{name: "500 with Not Found", status: http.StatusInternalServerError, contentType: "application/json",
			body: `{"chart":{"result":null,"error":{"code":"Not Found","description":"No data found, symbol may be delisted"}}}`},
		{name: "404 without Not Found", status: http.StatusNotFound, contentType: "application/json",
			body: `{"chart":{"result":[{}]}}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			client := yahooChartServer(t, tc.status, tc.contentType, tc.body)
			bars, err := client.GetOHLC("AAPL", start, &end)
			require.Nil(t, bars)
			require.Error(t, err)
			require.NotErrorIs(t, err, ErrNoBars)
		})
	}
}
