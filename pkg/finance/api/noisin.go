package api

import "strings"

// NoISINPrefix starts the finance service's text for an asked equity that
// the provider has no ISIN for: the GetAsset NotFound status message and the
// DownloadHistoricalData STATUS_ERROR. The rest is the requested symbol,
// never provider text. A retry can't fix it.
const NoISINPrefix = "no ISIN for "

// NoISINMessage is the finance service's no-ISIN text for symbol.
func NoISINMessage(symbol string) string {
	return NoISINPrefix + symbol
}

// IsNoISIN reports whether text is the finance service's no-ISIN text.
func IsNoISIN(text string) bool {
	return strings.HasPrefix(text, NoISINPrefix)
}
