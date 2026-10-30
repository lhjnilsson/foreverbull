package command

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// The provider in the invalid-ticker sentence comes from the allowlist; any
// other name becomes "The price provider", never the raw value.
func TestInvalidProviderTickerErrorCopy(t *testing.T) {
	t.Parallel()

	const rest = " returned an invalid ticker for XYZ, so its prices weren't imported. " +
		"Check the symbol or leave it out, and report it at https://github.com/lhjnilsson/foreverbull/issues."
	for provider, shown := range map[string]string{
		"yahoo":         "yahoo",
		"stooq":         "stooq",
		"alpaca":        "alpaca",
		"":              "The price provider",
		"polygon":       "The price provider",
		"yahoo\x1b[31m": "The price provider",
	} {
		require.EqualError(t, invalidProviderTickerError(provider, "XYZ"), shown+rest, "%q", provider)
	}
}

// The ingest's time-limit sentence names whole hours in hours and anything
// else in minutes, word for word.
func TestIngestTimeLimitCopy(t *testing.T) {
	t.Parallel()
	for limit, want := range map[time.Duration]string{
		2 * time.Hour: "The price import hit the 2-hour limit at AAPL. Prices imported before AAPL were kept. " +
			"Import fewer symbols at a time, or try again later.",
		time.Hour: "The price import hit the 1-hour limit at AAPL. Prices imported before AAPL were kept. " +
			"Import fewer symbols at a time, or try again later.",
		90 * time.Minute: "The price import hit the 90-minute limit at AAPL. Prices imported before AAPL were kept. " +
			"Import fewer symbols at a time, or try again later.",
	} {
		require.EqualError(t, ingestTimeLimitError(limit, "AAPL"), want, limit.String())
	}
}
