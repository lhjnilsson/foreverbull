package marketdata

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestBlockedErrorNamesProviderAndSuggestsTheOther(t *testing.T) {
	t.Parallel()
	require.EqualError(t, &BlockedError{Provider: "stooq", Symbol: "AAPL"},
		"Stooq blocked the download for AAPL (it returned a bot-check page instead of data). Retry with --provider yahoo.")
	require.EqualError(t, &BlockedError{Provider: "yahoo", Symbol: "MSFT"},
		"Yahoo blocked the download for MSFT (it returned a bot-check page instead of data). Retry with --provider stooq.")
}

func TestIsBlockedPage(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name        string
		contentType string
		body        string
		want        bool
	}{
		{name: "html content type", contentType: "text/html; charset=utf-8", body: "anything", want: true},
		{name: "markup without content type", body: "  <html></html>", want: true},
		{name: "access denied", contentType: "text/plain", body: "Access denied", want: true},
		{name: "access denied lowercase", contentType: "text/plain", body: "access denied.\n", want: true},
		{name: "stooq csv", contentType: "text/csv", body: "Date,Open,High,Low,Close,Volume\n", want: false},
		{name: "stooq no data", contentType: "text/plain", body: "No data", want: false},
		{name: "yahoo json", contentType: "application/json", body: `{"chart":{}}`, want: false},
		{name: "empty", contentType: "text/plain", body: "", want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, isBlockedPage(tc.contentType, []byte(tc.body)))
		})
	}
}

func TestYahooGetOHLCBotCheckIsBlockedError(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<!DOCTYPE html><html><body>Please verify you are human</body></html>"))
	}))
	t.Cleanup(srv.Close)

	client := &YahooClient{chartURL: srv.URL + "/"}
	start := time.Date(2021, 1, 4, 0, 0, 0, 0, time.UTC)
	end := time.Date(2021, 1, 6, 0, 0, 0, 0, time.UTC)
	bars, err := client.GetOHLC("aapl", start, &end)
	require.ErrorIs(t, err, ErrProviderBlocked)
	require.EqualError(t, err,
		"Yahoo blocked the download for AAPL (it returned a bot-check page instead of data). Retry with --provider stooq.")
	require.Nil(t, bars)
}
