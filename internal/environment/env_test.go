package environment

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSetupForTestFillsDefaultsWithoutDocker(t *testing.T) {
	t.Setenv(HTTPPort, "")
	t.Setenv(GRPCPort, "")
	t.Setenv(GrafanaURL, "")
	t.Setenv(PostgresURL, "")
	t.Setenv(NatsURL, "")
	t.Setenv(MinioURL, "")
	t.Setenv(DockerNetwork, "")
	t.Setenv(ServerAddress, "")
	t.Setenv(BacktestPortRangeStart, "")
	t.Setenv(BacktestPortRangeEnd, "")

	SetupForTest(t)

	require.Equal(t, HTTPPortDefault, GetHTTPPort())
	require.Equal(t, GRPCPortDefault, GetGRPCPort())
	require.Equal(t, GrafanaURLDefault, GetGrafanaURL())
	require.Equal(t, PostgresURLDefault, GetPostgresURL())
	require.Equal(t, NatsURLDefault, GetNATSURL())
	require.Equal(t, MinioURLDefault, GetMinioURL())
	require.Equal(t, DockerNetworkDefault, GetDockerNetworkName())
	require.Equal(t, ServerAddressDefault, GetServerAddress())
	require.Equal(t, 27000, GetBacktestPortRangeStart())
	require.Equal(t, 27015, GetBacktestPortRangeEnd())
}

func TestSetupForTestDoesNotOverrideSetValues(t *testing.T) {
	t.Setenv(HTTPPort, "12345")
	SetupForTest(t)
	require.Equal(t, "12345", GetHTTPPort())
}

func TestSetupFillsEmptyVars(t *testing.T) {
	t.Setenv(HTTPPort, "")
	require.NoError(t, Setup())
	require.Equal(t, HTTPPortDefault, GetHTTPPort())
	require.NotEmpty(t, os.Getenv(HTTPPort))
}

func TestTradingEnvDefaultsArePaper(t *testing.T) {
	for _, name := range []string{
		TradingEnabled,
		TradingProvider,
		TradingCurrency,
		TradingAllowLiveURL,
		TradingListingsFile,
		AlpacaBaseURL,
		AlpacaApiKey,
		AlpacaApiSecret,
		SaxoBaseURL,
		SaxoAccessToken,
		SaxoAccountKey,
		IGBaseURL,
		IGAPIKey,
		IGIdentifier,
		IGPassword,
		IBKRBaseURL,
		IBKRAccountID,
		IBKRAccountMode,
		IBKRAccessToken,
	} {
		t.Setenv(name, "")
	}

	SetupForTest(t)

	require.False(t, GetTradingEnabled())
	require.Equal(t, TradingProviderDefault, GetTradingProvider())
	require.Equal(t, TradingCurrencyDefault, GetTradingCurrency())
	require.False(t, GetTradingAllowLiveURL())
	require.Empty(t, GetTradingListingsFile())
	require.Equal(t, AlpacaBaseURLDefault, GetAlpacaBaseURL())
	require.Empty(t, GetAlpacaAPIKey())
	require.Empty(t, GetAlpacaAPISecret())
	require.Equal(t, SaxoBaseURLDefault, GetSaxoBaseURL())
	require.Contains(t, GetSaxoBaseURL(), "/sim/")
	require.Empty(t, GetSaxoAccessToken())
	require.Empty(t, GetSaxoAccountKey())
	require.Equal(t, IGBaseURLDefault, GetIGBaseURL())
	require.Contains(t, GetIGBaseURL(), "demo-api.ig.com")
	require.Empty(t, GetIGAPIKey())
	require.Empty(t, GetIGIdentifier())
	require.Empty(t, GetIGPassword())
	require.Equal(t, IBKRBaseURLDefault, GetIBKRBaseURL())
	require.Equal(t, "https://localhost:5000/v1/api", GetIBKRBaseURL())
	require.Empty(t, GetIBKRAccountID())
	require.Equal(t, IBKRAccountModeDefault, GetIBKRAccountMode())
	require.Empty(t, GetIBKRAccessToken())

	t.Setenv(TradingAllowLiveURL, "true")
	require.True(t, GetTradingAllowLiveURL())
	t.Setenv(TradingAllowLiveURL, "nope")
	require.False(t, GetTradingAllowLiveURL())
}

func TestGetTradingEnabledAndServerGRPCURL(t *testing.T) {
	t.Setenv(GRPCPort, "1234")
	require.Equal(t, "localhost:1234", GetServerGRPCURL())

	t.Setenv(TradingEnabled, "true")
	require.True(t, GetTradingEnabled())

	t.Setenv(TradingEnabled, "false")
	require.False(t, GetTradingEnabled())

	t.Setenv(TradingEnabled, "not-a-bool")
	require.False(t, GetTradingEnabled())
}
