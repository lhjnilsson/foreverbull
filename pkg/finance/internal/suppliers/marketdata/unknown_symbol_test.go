package marketdata

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestYahooGetAssetNotFoundIsUnknownSymbol(t *testing.T) {
	t.Parallel()
	notFound := `{"quoteSummary":{"result":null,"error":{"code":"Not Found","description":"Quote not found for symbol: FB"}}}`
	cases := []struct {
		name   string
		status int
		body   string
		want   string
	}{
		{name: "200 not found", status: http.StatusOK, body: notFound, want: "Quote not found for symbol: FB"},
		{name: "404 not found", status: http.StatusNotFound, body: notFound, want: "Quote not found for symbol: FB"},
		{name: "404 empty", status: http.StatusNotFound, body: `{"quoteSummary":{"result":[]}}`, want: "no results found"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			t.Cleanup(srv.Close)
			client := &YahooClient{crumb: "test", quoteURL: srv.URL + "/"}

			_, err := client.GetAsset(context.Background(), "FB")
			require.ErrorIs(t, err, ErrUnknownSymbol)
			require.EqualError(t, err, tc.want)
		})
	}
}

// A 200 with no error code and an empty result is not "Yahoo doesn't know
// this ticker". It stays the failed-request error it was.
func TestYahooGetAssetEmptyResultOn200IsNotUnknown(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"quoteSummary":{"result":[]}}`))
	}))
	t.Cleanup(srv.Close)
	client := &YahooClient{crumb: "test", quoteURL: srv.URL + "/"}

	_, err := client.GetAsset(context.Background(), "FB")
	require.Error(t, err)
	require.NotErrorIs(t, err, ErrUnknownSymbol)
	require.EqualError(t, err, "no results found")
}

func TestYahooGetAssetFailedRequestIsNotUnknown(t *testing.T) {
	t.Parallel()
	notFound := `{"quoteSummary":{"result":null,"error":{"code":"Not Found","description":"Quote not found for symbol: FB"}}}`

	t.Run("non-404", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(notFound))
		}))
		t.Cleanup(srv.Close)
		client := &YahooClient{crumb: "test", quoteURL: srv.URL + "/"}

		_, err := client.GetAsset(context.Background(), "FB")
		require.Error(t, err)
		require.NotErrorIs(t, err, ErrUnknownSymbol)
		require.Contains(t, err.Error(), "500")
	})

	t.Run("transport", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
		url := srv.URL
		srv.Close()
		client := &YahooClient{crumb: "test", quoteURL: url + "/"}

		_, err := client.GetAsset(context.Background(), "FB")
		require.Error(t, err)
		require.NotErrorIs(t, err, ErrUnknownSymbol)
	})

	t.Run("timeout", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
			time.Sleep(2 * time.Second)
		}))
		t.Cleanup(srv.Close)
		client := &YahooClient{crumb: "test", quoteURL: srv.URL + "/", httpClient: &http.Client{Timeout: time.Millisecond}}

		_, err := client.GetAsset(context.Background(), "FB")
		require.Error(t, err)
		require.NotErrorIs(t, err, ErrUnknownSymbol)
	})

	t.Run("bot check", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("<html>Not Found</html>"))
		}))
		t.Cleanup(srv.Close)
		client := &YahooClient{crumb: "test", quoteURL: srv.URL + "/"}

		_, err := client.GetAsset(context.Background(), "FB")
		require.Error(t, err)
		require.NotErrorIs(t, err, ErrUnknownSymbol)
		require.ErrorIs(t, err, ErrProviderBlocked)
	})
}
