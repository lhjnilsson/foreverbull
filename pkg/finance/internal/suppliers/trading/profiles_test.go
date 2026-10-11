package trading_test

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/lhjnilsson/foreverbull/pkg/finance/internal/suppliers/trading"
	"github.com/lhjnilsson/foreverbull/pkg/finance/supplier"
	"github.com/stretchr/testify/require"
)

func writeProfiles(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "profiles.yaml")
	require.NoError(t, os.WriteFile(path, []byte(body), 0o600))
	return path
}

func TestRegistryTwoProfilesBuildDifferentClients(t *testing.T) {
	transport := &recordingTransport{}
	path := writeProfiles(t, `
profiles:
  - name: alpaca-paper
    provider: alpaca
    apiKey: ""
    apiSecret: ""
  - name: saxo-sim
    provider: saxo
    accessToken: ""
    accountKey: ""
`)
	reg, err := trading.NewRegistry(path, true, trading.RegistryOptions{
		HTTPClient: &http.Client{Transport: transport},
	})
	require.NoError(t, err)

	alpacaClient, ok := reg.Get("alpaca-paper")
	require.True(t, ok)
	saxoClient, ok := reg.Get("saxo-sim")
	require.True(t, ok)
	again, ok := reg.Get("alpaca-paper")
	require.True(t, ok)
	require.Equal(t, reflect.ValueOf(alpacaClient).Pointer(), reflect.ValueOf(again).Pointer())
	require.NotEqual(t, reflect.ValueOf(alpacaClient).Pointer(), reflect.ValueOf(saxoClient).Pointer())

	_, missing := reg.Get("ibkr-paper")
	require.False(t, missing)

	_, err = alpacaClient.GetPortfolio(context.Background())
	require.ErrorIs(t, err, supplier.ErrUnavailable)
	require.Len(t, transport.hits, 1)
	require.Equal(t, "paper-api.alpaca.markets", transport.hits[0].host)
	require.Equal(t, "/v2/account", transport.hits[0].path)

	_, err = saxoClient.GetPortfolio(context.Background())
	require.ErrorIs(t, err, supplier.ErrUnavailable)
	require.Len(t, transport.hits, 2)
	require.Equal(t, "gateway.saxobank.com", transport.hits[1].host)
	require.Equal(t, "/sim/openapi/port/v1/accounts/me", transport.hits[1].path)
	for _, hit := range transport.hits {
		require.NotEqual(t, "api.alpaca.markets", hit.host)
	}
}

func TestRegistryRejectsBaseURLDuplicateUnknownAndIBKRLive(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		{
			name: "baseUrl",
			body: `
profiles:
  - name: alpaca-paper
    provider: alpaca
    baseUrl: https://api.alpaca.markets
`,
			want: "baseUrl",
		},
		{
			name: "duplicate",
			body: `
profiles:
  - name: alpaca-paper
    provider: alpaca
  - name: alpaca-paper
    provider: saxo
`,
			want: "duplicate",
		},
		{
			name: "unknown provider",
			body: `
profiles:
  - name: other
    provider: tradier
`,
			want: "unknown provider",
		},
		{
			name: "ibkr live",
			body: `
profiles:
  - name: ibkr-paper
    provider: ibkr
    accountId: DU123456
    accountMode: live
`,
			want: "live",
		},
		{
			name: "ibkr LIVE",
			body: `
profiles:
  - name: ibkr-paper
    provider: IBKR
    accountId: DU123456
    accountMode: LIVE
`,
			want: "live",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			transport := &recordingTransport{}
			path := writeProfiles(t, tc.body)
			_, err := trading.NewRegistry(path, true, trading.RegistryOptions{
				HTTPClient: &http.Client{Transport: transport},
			})
			require.Error(t, err)
			require.ErrorContains(t, err, tc.want)
			require.Empty(t, transport.hits)
		})
	}
}

func TestRegistryIBKRPaperUsesGatewayConstant(t *testing.T) {
	transport := &recordingTransport{}
	path := writeProfiles(t, `
profiles:
  - name: ibkr-paper
    provider: ibkr
    accountId: DU123456
`)
	reg, err := trading.NewRegistry(path, true, trading.RegistryOptions{
		HTTPClient: &http.Client{Transport: transport},
	})
	require.NoError(t, err)
	client, ok := reg.Get("ibkr-paper")
	require.True(t, ok)
	_, err = client.GetPortfolio(context.Background())
	require.ErrorIs(t, err, supplier.ErrUnavailable)
	require.Len(t, transport.hits, 1)
	require.Equal(t, "localhost:5000", transport.hits[0].host)
	require.Equal(t, "/v1/api/portfolio/accounts", transport.hits[0].path)
}

func TestRegistryEnabledMissingFile(t *testing.T) {
	_, err := trading.NewRegistry("", true, trading.RegistryOptions{})
	require.Error(t, err)
	require.ErrorContains(t, err, "TRADING_ENABLED")

	missing := filepath.Join(t.TempDir(), "missing.yaml")
	_, err = trading.NewRegistry(missing, true, trading.RegistryOptions{})
	require.Error(t, err)
}

func TestRegistryDisabledIgnoresMissingAndInvalidFile(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing.yaml")
	reg, err := trading.NewRegistry(missing, false, trading.RegistryOptions{})
	require.NoError(t, err)
	require.NotNil(t, reg)
	_, ok := reg.Get("alpaca-paper")
	require.False(t, ok)

	invalid := writeProfiles(t, `
profiles:
  - name: alpaca-paper
    provider: alpaca
    baseUrl: https://api.alpaca.markets
`)
	reg, err = trading.NewRegistry(invalid, false, trading.RegistryOptions{})
	require.NoError(t, err)
	_, ok = reg.Get("alpaca-paper")
	require.False(t, ok)

	reg, err = trading.NewRegistry("", false, trading.RegistryOptions{})
	require.NoError(t, err)
	require.NotNil(t, reg)
}
