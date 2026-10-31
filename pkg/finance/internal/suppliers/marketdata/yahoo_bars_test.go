package marketdata

import (
	"context"
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

// A null in Yahoo's price arrays is a missing session. Between two priced
// sessions it fails GetOHLC with the symbol and every such day, rather
// than leave a gap in the stored bars.
func TestYahooGetOHLCFailsOnANullBarInsideTheRange(t *testing.T) {
	t.Parallel()
	client := yahooFixtureServer(t, "testdata/yahoo_chart_null_bars.json")
	start := time.Date(2026, 5, 4, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 5, 9, 0, 0, 0, 0, time.UTC)

	bars, err := client.GetOHLC(context.Background(), "xyz", start, &end)
	require.Nil(t, bars)
	// 2026-05-05 is all null; 2026-05-07 has a null open only.
	require.EqualError(t, err, "yahoo sent no price for XYZ on 2026-05-05, 2026-05-07")
	var missing *MissingPriceError
	require.ErrorAs(t, err, &missing)
	require.Equal(t, "XYZ", missing.Symbol)
	require.Len(t, missing.Days, 2)
}

// A null at the start or the end of the answer fails too, naming the day:
// the bars would otherwise start late or end early with no word to the user.
func TestYahooGetOHLCFailsOnANullBarAtEitherEdge(t *testing.T) {
	t.Parallel()
	client := yahooFixtureServer(t, "testdata/yahoo_chart_null_edges.json")
	start := time.Date(2026, 5, 4, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 5, 9, 0, 0, 0, 0, time.UTC)

	bars, err := client.GetOHLC(context.Background(), "xyz", start, &end)
	require.Nil(t, bars)
	// 2026-05-04 is the leading null and 2026-05-08 the trailing one.
	require.EqualError(t, err, "yahoo sent no price for XYZ on 2026-05-04, 2026-05-08")
}

func TestYahooBarsFailsOnALeadingNull(t *testing.T) {
	t.Parallel()
	_, err := yahooBars(threeSessions(), []yahooQuote{quoteWithNullAt(3, 0)})
	var missing *MissingPriceError
	require.ErrorAs(t, err, &missing)
	require.Equal(t, "2026-05-04", MissingPriceDays(missing.Days))
}

// An old trailing null is a missing price, not today's unfinished session,
// which GetOHLC never asks for.
func TestYahooBarsFailsOnATrailingNull(t *testing.T) {
	t.Parallel()
	_, err := yahooBars(threeSessions(), []yahooQuote{quoteWithNullAt(3, 2)})
	var missing *MissingPriceError
	require.ErrorAs(t, err, &missing)
	require.Equal(t, "2026-05-06", MissingPriceDays(missing.Days))
}

// 2026-05-04, 05 and 06 at the 09:30 New York open.
func threeSessions() []int64 {
	return []int64{1777901400, 1777987800, 1778074200}
}

func quoteWithNullAt(count, index int) yahooQuote {
	price := func(v float64) *float64 { return &v }
	quote := yahooQuote{}
	for i := 0; i < count; i++ {
		value := price(10)
		if i == index {
			value = nil
		}
		quote.Open = append(quote.Open, value)
		quote.High = append(quote.High, value)
		quote.Low = append(quote.Low, value)
		quote.Close = append(quote.Close, value)
		quote.Volume = append(quote.Volume, nil)
	}
	return quote
}

func TestYahooBarsFailsOnZeroAndNegativePrices(t *testing.T) {
	t.Parallel()
	price := func(v float64) *float64 { return &v }
	volume := uint64(5)
	quote := yahooQuote{
		Open:   []*float64{price(1), nil, price(0), price(1), price(1)},
		High:   []*float64{price(1), nil, price(1), price(1), price(1)},
		Low:    []*float64{price(1), nil, price(1), price(-1), price(1)},
		Close:  []*float64{price(1), nil, price(1), price(1), price(1)},
		Volume: []*uint64{&volume, nil, &volume, &volume, &volume},
	}
	timestamps := []int64{1777901400, 1777987800, 1778074200, 1778160600, 1778247000}

	bars, err := yahooBars(timestamps, []yahooQuote{quote})
	require.Nil(t, bars)
	var missing *MissingPriceError
	require.ErrorAs(t, err, &missing)
	require.Equal(t, "2026-05-05, 2026-05-06, 2026-05-07", MissingPriceDays(missing.Days),
		"null, zero, and negative prices are all missing bars")
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
	_, err := yahooBars([]int64{1777901400, 1777987800}, []yahooQuote{quote})
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
	bars, err := notFound.GetOHLC(context.Background(), "NON_EXISTING", start, &end)
	require.Nil(t, bars)
	require.ErrorIs(t, err, ErrNoBars)
	require.EqualError(t, err, "fail to get OHLC data for symbol NON_EXISTING: No data found, symbol may be delisted")

	// 404 with the Not Found chart error is the same answer.
	notFound404 := yahooChartServer(t, http.StatusNotFound, "application/json",
		`{"chart":{"result":null,"error":{"code":"Not Found","description":"No data found, symbol may be delisted"}}}`)
	_, err = notFound404.GetOHLC(context.Background(), "NON_EXISTING", start, &end)
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
			bars, err := client.GetOHLC(context.Background(), "AAPL", start, &end)
			require.Nil(t, bars)
			require.Error(t, err)
			require.NotErrorIs(t, err, ErrNoBars)
		})
	}
}
