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
	require.Equal(t, ServerAddressDefault, GetServerAddress())
	portRange, err := GetBacktestPortRange()
	require.NoError(t, err)
	require.Equal(t, PortRange{Start: 27000, End: 27015}, portRange)
}

func TestGetBacktestPortRangeRejectsBadValues(t *testing.T) {
	cases := []struct {
		name  string
		start string
		end   string
		want  string
	}{
		{name: "start not a number", start: "abc", end: "27015", want: "BACKTEST_PORT_RANGE_START=\"abc\" is not a port number"},
		{name: "end empty", start: "27000", end: "", want: "BACKTEST_PORT_RANGE_END=\"\" is not a port number"},
		{name: "start zero", start: "0", end: "27015", want: "BACKTEST_PORT_RANGE_START=0 is outside 1-65535"},
		{name: "end above 65535", start: "27000", end: "70000", want: "BACKTEST_PORT_RANGE_END=70000 is outside 1-65535"},
		{name: "start above end", start: "27016", end: "27015", want: "BACKTEST_PORT_RANGE_START (27016) is above BACKTEST_PORT_RANGE_END (27015)"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(BacktestPortRangeStart, tc.start)
			t.Setenv(BacktestPortRangeEnd, tc.end)

			_, err := GetBacktestPortRange()
			require.ErrorContains(t, err, tc.want)
		})
	}
}

func TestGetBacktestPortRangeSinglePortIsInclusive(t *testing.T) {
	t.Setenv(BacktestPortRangeStart, "27000")
	t.Setenv(BacktestPortRangeEnd, "27000")

	portRange, err := GetBacktestPortRange()
	require.NoError(t, err)
	require.True(t, portRange.Contains(27000))
	require.False(t, portRange.Contains(27001))
}

func TestSetupFailsOnBadPortRange(t *testing.T) {
	t.Setenv(BacktestPortRangeStart, "not-a-port")

	err := Setup()
	require.ErrorContains(t, err, "BACKTEST_PORT_RANGE_START")
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

func TestTradingEnabledDefaultsFalse(t *testing.T) {
	t.Setenv(TradingEnabled, "")
	t.Setenv(TradingProfilesFile, "")
	t.Setenv(TradingListingsFile, "")

	SetupForTest(t)

	require.False(t, GetTradingEnabled())
	require.Empty(t, GetTradingProfilesFile())
	require.Empty(t, GetTradingListingsFile())

	t.Setenv(TradingProfilesFile, "/tmp/profiles.yaml")
	require.Equal(t, "/tmp/profiles.yaml", GetTradingProfilesFile())
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
