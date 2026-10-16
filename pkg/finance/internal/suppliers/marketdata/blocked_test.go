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
	require.EqualError(t, &BlockedError{Provider: "stooq", Symbol: "AAPL", Refused: true},
		"Stooq blocked the download for AAPL (it refused the request instead of returning data). Retry with --provider yahoo.")
	require.EqualError(t, &BlockedError{Provider: "yahoo", Symbol: "MSFT", Refused: true},
		"Yahoo blocked the download for MSFT (it refused the request instead of returning data). Retry with --provider stooq.")
}

func TestClassifyBlock(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name        string
		contentType string
		body        string
		want        blockKind
	}{
		{name: "html content type", contentType: "text/html; charset=utf-8", body: "anything", want: blockedByBotCheck},
		{name: "markup without content type", body: "  <html></html>", want: blockedByBotCheck},
		{name: "access denied", contentType: "text/plain", body: "Access denied", want: blockedByRefusal},
		{name: "access denied lowercase", contentType: "text/plain", body: "access denied.\n", want: blockedByRefusal},
		{name: "stooq csv", contentType: "text/csv", body: "Date,Open,High,Low,Close,Volume\n", want: notBlocked},
		{name: "stooq no data", contentType: "text/plain", body: "No data", want: notBlocked},
		{name: "yahoo json", contentType: "application/json", body: `{"chart":{}}`, want: notBlocked},
		{name: "empty", contentType: "text/plain", body: "", want: notBlocked},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, classifyBlock(tc.contentType, []byte(tc.body)))
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
