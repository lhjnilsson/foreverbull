package api

import "strings"

// UnknownSymbolPrefix starts the finance service's text for a ticker Yahoo
// answered it does not know: the GetAsset NotFound status message and the
// DownloadHistoricalData STATUS_ERROR. The rest is the requested symbol,
// never Yahoo's own text. A retry can't fix it, and the lookup is Yahoo
// for every price provider.
const UnknownSymbolPrefix = "unknown symbol "

// UnknownSymbolMessage is the finance service's unknown-symbol text for symbol.
func UnknownSymbolMessage(symbol string) string {
	return UnknownSymbolPrefix + symbol
}

// IsUnknownSymbol reports whether text is the finance service's unknown-symbol text.
func IsUnknownSymbol(text string) bool {
	return strings.HasPrefix(text, UnknownSymbolPrefix)
}
