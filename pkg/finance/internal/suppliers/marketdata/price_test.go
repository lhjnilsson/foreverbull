package marketdata

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizeProvider(t *testing.T) {
	t.Parallel()

	got, err := NormalizeProvider("")
	require.NoError(t, err)
	require.Equal(t, "yahoo", got)

	got, err = NormalizeProvider("  Yahoo ")
	require.NoError(t, err)
	require.Equal(t, "yahoo", got)

	got, err = NormalizeProvider("stooq")
	require.NoError(t, err)
	require.Equal(t, "stooq", got)

	got, err = NormalizeProvider("alpaca")
	require.NoError(t, err)
	require.Equal(t, "alpaca", got)

	_, err = NormalizeProvider("binance")
	require.ErrorContains(t, err, "unknown price provider")
	require.ErrorContains(t, err, "valid providers: yahoo, stooq, alpaca")
}

func TestShownProvider(t *testing.T) {
	t.Parallel()

	for provider, want := range map[string]string{
		"yahoo":             "yahoo",
		" Stooq ":           "stooq",
		"ALPACA":            "alpaca",
		"":                  UnknownProviderName,
		"  ":                UnknownProviderName,
		"polygon":           UnknownProviderName,
		"yahoo\x1b[31m":     UnknownProviderName,
		"<b>yahoo</b>":      UnknownProviderName,
		"yahoo; drop table": UnknownProviderName,
	} {
		require.Equal(t, want, ShownProvider(provider), "%q", provider)
	}
}
