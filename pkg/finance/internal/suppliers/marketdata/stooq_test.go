package marketdata_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/lhjnilsson/foreverbull/pkg/finance/internal/suppliers/marketdata"
	"github.com/stretchr/testify/require"
)

const priceDelta = 1e-9

type stooqReq struct {
	path, host, s, i, d1, d2 string
}

func TestStooqMetadataUnimplemented(t *testing.T) {
	t.Parallel()
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		called = true
	}))
	t.Cleanup(srv.Close)

	client := newStooqClient(t, srv.URL)

	_, err := client.GetAsset("AAPL")
	require.ErrorIs(t, err, marketdata.ErrStooqMetadata)
	_, err = client.SearchAsset("AAPL")
	require.ErrorIs(t, err, marketdata.ErrStooqMetadata)
	_, err = client.GetIndex("^DJI")
	require.ErrorIs(t, err, marketdata.ErrStooqMetadata)
	require.False(t, called)
}

func TestGetOHLCAAPLMapsToStooqUS(t *testing.T) {
	t.Parallel()
	var got stooqReq
	srv := serveStooq(t, readFixture(t, "aapl.us.csv"), http.StatusOK, &got)

	client := newStooqClient(t, srv.URL)
	start := date(t, "2021-01-04")
	end := date(t, "2021-01-06")
	bars, err := client.GetOHLC("AAPL", start, &end)
	require.NoError(t, err)
	require.Equal(t, "/q/d/l/", got.path)
	require.Equal(t, "aapl.us", got.s)
	require.Equal(t, "d", got.i)
	require.Equal(t, "20210104", got.d1)
	require.Equal(t, "20210106", got.d2)
	require.Len(t, bars, 3)
	require.Equal(t, "AAPL", bars[0].Symbol)
	require.Equal(t, date(t, "2021-01-04").Unix(), bars[0].Timestamp.AsTime().Unix())
	require.InDelta(t, 133.52, bars[0].Open, priceDelta)
	require.InDelta(t, 133.6115, bars[0].High, priceDelta)
	require.InDelta(t, 126.76, bars[0].Low, priceDelta)
	require.InDelta(t, 129.41, bars[0].Close, priceDelta)
	require.Equal(t, uint64(143301887), bars[0].Volume)
	require.Equal(t, "AAPL", bars[2].Symbol)
	require.InDelta(t, 126.6, bars[2].Close, priceDelta)
}

func TestGetOHLCAcceptsStooqCodeAsCanonicalAAPL(t *testing.T) {
	t.Parallel()
	var got stooqReq
	srv := serveStooq(t, readFixture(t, "aapl.us.csv"), http.StatusOK, &got)

	client := newStooqClient(t, srv.URL)
	start := date(t, "2021-01-04")
	end := date(t, "2021-01-06")
	bars, err := client.GetOHLC("aapl.us", start, &end)
	require.NoError(t, err)
	require.Equal(t, "aapl.us", got.s)
	require.NotEmpty(t, bars)
	for _, bar := range bars {
		require.Equal(t, "AAPL", bar.Symbol)
	}
}

func TestGetOHLCSPYMapsToStooqUS(t *testing.T) {
	t.Parallel()
	var got stooqReq
	srv := serveStooq(t, readFixture(t, "spy.us.csv"), http.StatusOK, &got)

	client := newStooqClient(t, srv.URL)
	start := date(t, "2021-01-04")
	end := date(t, "2021-01-05")
	bars, err := client.GetOHLC("spy", start, &end)
	require.NoError(t, err)
	require.Equal(t, "spy.us", got.s)
	require.Len(t, bars, 2)
	require.Equal(t, "SPY", bars[0].Symbol)
	require.InDelta(t, 368.79, bars[0].Close, priceDelta)
	require.Equal(t, uint64(110210800), bars[0].Volume)
}

func TestGetOHLCUnknownSymbolFallsBackToUS(t *testing.T) {
	t.Parallel()
	var got stooqReq
	srv := serveStooq(t, readFixture(t, "bbby.us.csv"), http.StatusOK, &got)

	client := newStooqClient(t, srv.URL)
	start := date(t, "2023-08-01")
	end := date(t, "2023-10-01")
	bars, err := client.GetOHLC("XYZ", start, &end)
	require.NoError(t, err)
	require.Equal(t, "xyz.us", got.s)
	require.NotEmpty(t, bars)
	require.Equal(t, "XYZ", bars[0].Symbol)
}

func TestGetOHLCDelistedSeriesParses(t *testing.T) {
	t.Parallel()
	srv := serveStooq(t, readFixture(t, "bbby.us.csv"), http.StatusOK, nil)

	client := newStooqClient(t, srv.URL)
	start := date(t, "2023-01-01")
	end := date(t, "2024-01-01")
	bars, err := client.GetOHLC("BBBY", start, &end)
	require.NoError(t, err)
	require.Len(t, bars, 4)
	require.Equal(t, "BBBY", bars[0].Symbol)
	require.Equal(t, date(t, "2023-08-15").Unix(), bars[0].Timestamp.AsTime().Unix())
	last := bars[len(bars)-1]
	require.Equal(t, date(t, "2023-09-29").Unix(), last.Timestamp.AsTime().Unix())
	require.InDelta(t, 0.05, last.Close, priceDelta)
	require.Equal(t, uint64(45881209), last.Volume)
}

func TestGetOHLCEmptyAndMissingAreErrors(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		status     int
		fixture    string
		body       string
		wantSubstr string
	}{
		{name: "empty body", status: http.StatusOK, fixture: "empty.csv", wantSubstr: "empty response"},
		{name: "header only", status: http.StatusOK, fixture: "header_only.csv", wantSubstr: "empty response"},
		{name: "no data", status: http.StatusOK, fixture: "no_data.txt", wantSubstr: "missing file"},
		{name: "malformed", status: http.StatusOK, fixture: "malformed.csv", wantSubstr: "unparsable CSV"},
		{name: "http 404", status: http.StatusNotFound, body: "not found", wantSubstr: "missing file"},
		{name: "http 500", status: http.StatusInternalServerError, body: "oops", wantSubstr: "missing file"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			body := []byte(tc.body)
			if tc.fixture != "" {
				body = readFixture(t, tc.fixture)
			}
			srv := serveStooq(t, body, tc.status, nil)

			client := newStooqClient(t, srv.URL)
			start := date(t, "2021-01-04")
			end := date(t, "2021-01-06")
			bars, err := client.GetOHLC("AAPL", start, &end)
			require.Error(t, err)
			require.ErrorContains(t, err, tc.wantSubstr)
			require.Nil(t, bars)
		})
	}
}

func TestGetOHLCBotCheckIsBlockedError(t *testing.T) {
	t.Parallel()

	const botCheck = "Stooq blocked the download for AAPL (it returned a bot-check page instead of data). Retry with --provider yahoo."
	const refused = "Stooq blocked the download for AAPL (it refused the request instead of returning data). Retry with --provider yahoo."
	cases := []struct {
		name        string
		status      int
		contentType string
		body        string
		want        string
	}{
		{name: "html bot check", status: http.StatusOK, contentType: "text/html; charset=utf-8", body: "<!DOCTYPE html><html><body>Checking your browser</body></html>", want: botCheck},
		{name: "html sent as csv", status: http.StatusOK, contentType: "text/csv", body: "\n  <html><body>captcha</body></html>", want: botCheck},
		{name: "html 403", status: http.StatusForbidden, contentType: "text/html", body: "<html>Forbidden</html>", want: botCheck},
		{name: "plain access denied", status: http.StatusOK, contentType: "text/plain", body: "Access denied", want: refused},
		{name: "plain access denied with newline", status: http.StatusOK, contentType: "text/plain; charset=utf-8", body: "Access denied\n", want: refused},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			srv := serveStooqAs(t, []byte(tc.body), tc.status, tc.contentType)

			client := newStooqClient(t, srv.URL)
			start := date(t, "2021-01-04")
			end := date(t, "2021-01-06")
			bars, err := client.GetOHLC("AAPL", start, &end)
			require.ErrorIs(t, err, marketdata.ErrProviderBlocked)
			var blocked *marketdata.BlockedError
			require.ErrorAs(t, err, &blocked)
			require.Equal(t, "stooq", blocked.Provider)
			require.Equal(t, "AAPL", blocked.Symbol)
			require.EqualError(t, err, tc.want)
			require.Nil(t, bars)
		})
	}
}

func TestGetOHLCMissingFileIsNotBlocked(t *testing.T) {
	t.Parallel()
	srv := serveStooq(t, readFixture(t, "no_data.txt"), http.StatusOK, nil)

	client := newStooqClient(t, srv.URL)
	start := date(t, "2021-01-04")
	end := date(t, "2021-01-06")
	_, err := client.GetOHLC("AAPL", start, &end)
	require.Error(t, err)
	require.NotErrorIs(t, err, marketdata.ErrProviderBlocked)
}

func TestGetOHLCEmptySymbol(t *testing.T) {
	t.Parallel()
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		called = true
	}))
	t.Cleanup(srv.Close)

	client := newStooqClient(t, srv.URL)
	_, err := client.GetOHLC("  ", date(t, "2021-01-04"), nil)
	require.ErrorContains(t, err, "empty symbol")
	require.False(t, called)
}

func TestGetOHLCNilEndUsesToday(t *testing.T) {
	t.Parallel()
	var got stooqReq
	srv := serveStooq(t, readFixture(t, "aapl.us.csv"), http.StatusOK, &got)

	client := newStooqClient(t, srv.URL)
	_, err := client.GetOHLC("AAPL", date(t, "2021-01-04"), nil)
	require.NoError(t, err)
	require.Equal(t, time.Now().UTC().Format("20060102"), got.d2)
}

func TestWithStooqSymbolMapFile(t *testing.T) {
	t.Parallel()
	var got stooqReq
	srv := serveStooq(t, readFixture(t, "aapl.us.csv"), http.StatusOK, &got)

	client, err := marketdata.NewStooqClient(
		marketdata.WithStooqBaseURL(srv.URL),
		marketdata.WithStooqSymbolMapFile(filepath.Join("testdata", "extra_symbols.csv")),
	)
	require.NoError(t, err)

	start := date(t, "2021-01-04")
	end := date(t, "2021-01-06")
	bars, err := client.GetOHLC("FOO", start, &end)
	require.NoError(t, err)
	require.Equal(t, "foo.us", got.s)
	require.Equal(t, "FOO", bars[0].Symbol)
}

func TestWithStooqSymbolMapOverridesBuiltin(t *testing.T) {
	t.Parallel()
	var got stooqReq
	srv := serveStooq(t, readFixture(t, "aapl.us.csv"), http.StatusOK, &got)

	client, err := marketdata.NewStooqClient(
		marketdata.WithStooqBaseURL(srv.URL),
		marketdata.WithStooqSymbolMap(map[string]string{"AAPL": "aapl.ny"}),
	)
	require.NoError(t, err)

	start := date(t, "2021-01-04")
	end := date(t, "2021-01-06")
	bars, err := client.GetOHLC("AAPL", start, &end)
	require.NoError(t, err)
	require.Equal(t, "aapl.ny", got.s)
	require.Equal(t, "AAPL", bars[0].Symbol)
}

func TestNewStooqClientMissingMapFile(t *testing.T) {
	t.Parallel()
	_, err := marketdata.NewStooqClient(
		marketdata.WithStooqSymbolMapFile("testdata/does-not-exist.csv"),
	)
	require.ErrorIs(t, err, os.ErrNotExist)
}

func TestGetOHLCDoesNotHitLiveHost(t *testing.T) {
	t.Parallel()
	var got stooqReq
	srv := serveStooq(t, readFixture(t, "aapl.us.csv"), http.StatusOK, &got)

	client := newStooqClient(t, srv.URL)
	start := date(t, "2021-01-04")
	end := date(t, "2021-01-06")
	_, err := client.GetOHLC("AAPL", start, &end)
	require.NoError(t, err)
	require.NotContains(t, got.host, "stooq.com")
}

func newStooqClient(t *testing.T, baseURL string) *marketdata.StooqClient {
	t.Helper()
	client, err := marketdata.NewStooqClient(marketdata.WithStooqBaseURL(baseURL))
	require.NoError(t, err)
	return client
}

func serveStooqAs(t *testing.T, body []byte, status int, contentType string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", contentType)
		w.WriteHeader(status)
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func serveStooq(t *testing.T, body []byte, status int, rec *stooqReq) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if rec != nil {
			rec.path = r.URL.Path
			rec.host = r.Host
			rec.s = r.URL.Query().Get("s")
			rec.i = r.URL.Query().Get("i")
			rec.d1 = r.URL.Query().Get("d1")
			rec.d2 = r.URL.Query().Get("d2")
		}
		w.Header().Set("Content-Type", "text/csv")
		w.WriteHeader(status)
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	body, err := os.ReadFile(filepath.Join("testdata", name))
	require.NoError(t, err)
	return body
}

func date(t *testing.T, s string) time.Time {
	t.Helper()
	d, err := time.Parse("2006-01-02", s)
	require.NoError(t, err)
	return d
}
