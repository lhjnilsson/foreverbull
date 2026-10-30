package command

import (
	"testing"

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
