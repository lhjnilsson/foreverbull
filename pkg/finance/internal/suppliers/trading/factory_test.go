package trading_test

import (
	"context"
	"errors"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/lhjnilsson/foreverbull/pkg/finance/internal/suppliers/trading"
	"github.com/lhjnilsson/foreverbull/pkg/finance/supplier"
	pb "github.com/lhjnilsson/foreverbull/pkg/pb/finance"
	"github.com/stretchr/testify/require"
)

type factoryHit struct {
	host string
	path string
}

// recordingTransport never dials. login answers the one IG session POST.
type recordingTransport struct {
	hits  []factoryHit
	login bool
}

func (r *recordingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	r.hits = append(r.hits, factoryHit{host: req.URL.Host, path: req.URL.Path})
	if r.login {
		return igSessionResponse(), nil
	}
	return nil, errors.New("stopped")
}

func TestFactoryPaperURLsWithoutLiveFlag(t *testing.T) {
	cases := []struct {
		name  string
		login bool
		cfg   trading.Config
		host  string
		path  string
	}{
		{
			name: "alpaca",
			cfg: trading.Config{
				Provider:      "alpaca",
				AlpacaBaseURL: "https://paper-api.alpaca.markets",
			},
			host: "paper-api.alpaca.markets",
			path: "/v2/account",
		},
		{
			name: "saxo",
			cfg: trading.Config{
				Provider:    "saxo",
				SaxoBaseURL: "https://gateway.saxobank.com/sim/openapi",
			},
			host: "gateway.saxobank.com",
			path: "/sim/openapi/port/v1/accounts/me",
		},
		{
			name:  "ig",
			login: true,
			cfg: trading.Config{
				Provider:  "ig",
				IGBaseURL: "https://demo-api.ig.com/gateway/deal",
			},
			host: "demo-api.ig.com",
			path: "/gateway/deal/session",
		},
		{
			name: "ibkr",
			cfg: trading.Config{
				Provider:      "ibkr",
				IBKRBaseURL:   "https://localhost:5000/v1/api",
				IBKRAccountID: ibkrAccountID,
			},
			host: "localhost:5000",
			path: "/v1/api/portfolio/accounts",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			transport := &recordingTransport{login: tc.login}
			cfg := tc.cfg
			cfg.HTTPClient = &http.Client{Transport: transport}
			got, err := trading.New(cfg)
			require.NoError(t, err)
			require.IsType(t, &trading.Wrapper{}, got)

			if tc.login {
				require.Len(t, transport.hits, 1)
				require.Equal(t, tc.host, transport.hits[0].host)
				require.Equal(t, tc.path, transport.hits[0].path)
				return
			}

			require.Empty(t, transport.hits)
			_, err = got.GetPortfolio(context.Background())
			require.ErrorIs(t, err, supplier.ErrUnavailable)
			require.Len(t, transport.hits, 1)
			require.Equal(t, tc.host, transport.hits[0].host)
			require.Equal(t, tc.path, transport.hits[0].path)
		})
	}
}

func TestFactoryLiveURLRefusedUnlessAllowed(t *testing.T) {
	cases := []struct {
		name  string
		login bool
		cfg   func(allow bool) trading.Config
		host  string
		path  string
	}{
		{
			name: "alpaca",
			cfg: func(allow bool) trading.Config {
				return trading.Config{
					Provider:      "alpaca",
					AllowLiveURL:  allow,
					AlpacaBaseURL: "https://api.alpaca.markets",
				}
			},
			host: "api.alpaca.markets",
			path: "/v2/account",
		},
		{
			name: "saxo",
			cfg: func(allow bool) trading.Config {
				return trading.Config{
					Provider:     "saxo",
					AllowLiveURL: allow,
					SaxoBaseURL:  "https://gateway.saxobank.com/openapi",
				}
			},
			host: "gateway.saxobank.com",
			path: "/openapi/port/v1/accounts/me",
		},
		{
			name: "saxo-cleaned-sim",
			cfg: func(allow bool) trading.Config {
				return trading.Config{
					Provider:     "saxo",
					AllowLiveURL: allow,
					SaxoBaseURL:  "https://gateway.saxobank.com/sim/../openapi",
				}
			},
			host: "gateway.saxobank.com",
			path: "/openapi/port/v1/accounts/me",
		},
		{
			name:  "ig",
			login: true,
			cfg: func(allow bool) trading.Config {
				return trading.Config{
					Provider:     "ig",
					AllowLiveURL: allow,
					IGBaseURL:    "https://api.ig.com/gateway/deal",
				}
			},
			host: "api.ig.com",
			path: "/gateway/deal/session",
		},
		{
			name: "ibkr",
			cfg: func(allow bool) trading.Config {
				return trading.Config{
					Provider:        "ibkr",
					AllowLiveURL:    allow,
					IBKRBaseURL:     "https://localhost:5000/v1/api",
					IBKRAccountID:   ibkrAccountID,
					IBKRAccountMode: "LIVE",
				}
			},
			host: "localhost:5000",
			path: "/v1/api/portfolio/accounts",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			denied := &recordingTransport{}
			cfg := tc.cfg(false)
			cfg.HTTPClient = &http.Client{Transport: denied}
			_, err := trading.New(cfg)
			require.Error(t, err)
			require.ErrorContains(t, err, "refused")
			require.NotErrorIs(t, err, supplier.ErrUnavailable)
			require.Empty(t, denied.hits)

			allowedTransport := &recordingTransport{login: tc.login}
			cfg = tc.cfg(true)
			cfg.HTTPClient = &http.Client{Transport: allowedTransport}
			got, err := trading.New(cfg)
			require.NoError(t, err)
			require.IsType(t, &trading.Wrapper{}, got)
			if tc.login {
				require.Len(t, allowedTransport.hits, 1)
				require.Equal(t, tc.host, allowedTransport.hits[0].host)
				require.Equal(t, tc.path, allowedTransport.hits[0].path)
				return
			}
			require.Empty(t, allowedTransport.hits)
			_, err = got.GetPortfolio(context.Background())
			require.ErrorIs(t, err, supplier.ErrUnavailable)
			require.Len(t, allowedTransport.hits, 1)
			require.Equal(t, tc.host, allowedTransport.hits[0].host)
			require.Equal(t, tc.path, allowedTransport.hits[0].path)
		})
	}
}

func TestFactoryUnknownProvider(t *testing.T) {
	transport := &recordingTransport{}
	_, err := trading.New(trading.Config{
		Provider:   "tradier",
		HTTPClient: &http.Client{Transport: transport},
	})
	require.Error(t, err)
	require.ErrorContains(t, err, "unknown trading provider")
	require.Empty(t, transport.hits)

	_, err = trading.New(trading.Config{Provider: "  "})
	require.Error(t, err)
	require.ErrorContains(t, err, "unknown trading provider")
}

func TestFactoryWrapperFillsISIN(t *testing.T) {
	transport := &recordingTransport{}
	dir := trading.NewMemoryDirectory(trading.SymbolAlias{
		ISIN:   aaplISIN,
		Symbol: "AAPL",
	})
	got, err := trading.New(trading.Config{
		Provider:      "alpaca",
		AlpacaBaseURL: "https://paper-api.alpaca.markets",
		HTTPClient:    &http.Client{Transport: transport},
		Directory:     dir,
	})
	require.NoError(t, err)

	_, err = got.PlaceOrder(context.Background(), &pb.Order{
		Symbol: "AAPL",
		Amount: 10,
		Type:   pb.OrderType_ORDER_TYPE_MARKET,
	})
	require.ErrorIs(t, err, supplier.ErrUnavailable)
	require.NotErrorIs(t, err, supplier.ErrNotTradable)
	require.Len(t, transport.hits, 1)
	require.Equal(t, "/v2/orders", transport.hits[0].path)

	ambiguous := &recordingTransport{}
	two := trading.NewMemoryDirectory(
		trading.SymbolAlias{ISIN: aaplISIN, Symbol: "DUAL"},
		trading.SymbolAlias{ISIN: "US5949181045", Symbol: "DUAL"},
	)
	got, err = trading.New(trading.Config{
		Provider:      "alpaca",
		AlpacaBaseURL: "https://paper-api.alpaca.markets",
		HTTPClient:    &http.Client{Transport: ambiguous},
		Directory:     two,
	})
	require.NoError(t, err)
	_, err = got.PlaceOrder(context.Background(), &pb.Order{
		Symbol: "DUAL",
		Amount: 1,
		Type:   pb.OrderType_ORDER_TYPE_MARKET,
	})
	require.ErrorIs(t, err, supplier.ErrNotTradable)
	require.Empty(t, ambiguous.hits)
}

func TestFactoryMissingListingsFile(t *testing.T) {
	transport := &recordingTransport{}
	_, err := trading.New(trading.Config{
		Provider:     "alpaca",
		ListingsFile: filepath.Join(t.TempDir(), "listings.json"),
		HTTPClient:   &http.Client{Transport: transport},
	})
	require.Error(t, err)
	require.Empty(t, transport.hits)
}
