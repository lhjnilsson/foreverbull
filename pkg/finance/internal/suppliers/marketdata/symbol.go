package marketdata

import (
	"github.com/lhjnilsson/foreverbull/pkg/finance/internal/mapping"
)

// parseShareClass reports a separated share class (BRK.B, BRK-B). A joined
// iShares ticker is not one until a Classes has learned it. An exchange
// suffix such as VOD.L is not one.
func parseShareClass(symbol string) bool {
	_, _, ok := mapping.ParseShareClass(symbol)
	return ok
}

// StoredSymbol is the one name a share class is stored under. BRK.B, BRK-B,
// BRK/B and BRK B all store as BRKB, which is also the joined iShares
// ticker, so the spellings are one row. Any other symbol, including an
// exchange suffix such as VOD.L, is returned exactly as typed.
func StoredSymbol(symbol string) string {
	return mapping.Stored(symbol)
}

// yahooSymbol is the ticker Yahoo knows a separated share class by.
// A joined iShares ticker is unchanged until Classes has learned it.
func yahooSymbol(symbol string) string {
	return mapping.Yahoo(symbol)
}

// LearnShareClasses teaches joined tickers from separated spellings such as
// BRK.B. The broker's price clients read the same set.
func LearnShareClasses(symbols ...string) {
	mapping.Process().Learn(symbols...)
}

// LearnJoinedShareClass teaches asked when a provider symbol for its ISIN
// is the separated spelling of that same ticker.
func LearnJoinedShareClass(asked string, hits ...string) {
	mapping.Process().LearnIfJoined(asked, hits)
}

// JoinedShareClass reports an iShares-style ticker that might be a share
// class written without a separator.
func JoinedShareClass(symbol string) bool {
	return mapping.MightBeJoinedClass(symbol)
}

// YahooSymbol is the ticker Yahoo is asked for, including a joined ticker
// the process has learned.
func YahooSymbol(symbol string) string {
	return mapping.FormatYahoo(mapping.Process(), symbol)
}

// alpacaSymbol is the ticker Alpaca knows the symbol by.
func alpacaSymbol(symbol string) string {
	return mapping.Alpaca(symbol)
}

// stooqShareClassCode is Stooq's daily code for a separated share class.
func stooqShareClassCode(symbol, exchange string) (string, bool) {
	return mapping.StooqShareClass(symbol, exchange)
}
