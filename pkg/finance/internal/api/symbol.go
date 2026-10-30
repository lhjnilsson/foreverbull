package api

import (
	"regexp"
	"strconv"
)

// symbolPattern is the ticker shape a download accepts, stores bars under, and
// names in a client message. The backtest service shows a refused-bar
// sentence only when its symbol has this shape, so a provider or caller
// string outside it (spaces, control bytes, markup, 33+ bytes) is refused
// here instead of reaching a user.
var symbolPattern = regexp.MustCompile(`^[A-Za-z0-9.^=_-]{1,32}$`)

func validSymbol(symbol string) bool {
	return symbolPattern.MatchString(symbol)
}

// invalidSymbolMessage refuses a download request with a symbol outside
// symbolPattern. It does not repeat the symbol, because that is the text
// that failed the check.
const invalidSymbolMessage = "a symbol isn't a valid ticker (use 1 to 32 letters, digits, or . ^ = _ -)"

// maxLoggedSymbol caps how much of a refused symbol goes into a log line.
const maxLoggedSymbol = 64

// loggedSymbol quotes a refused symbol for a log field, so control bytes stay
// escaped and a long provider string can't flood the log.
func loggedSymbol(symbol string) string {
	if len(symbol) > maxLoggedSymbol {
		symbol = symbol[:maxLoggedSymbol]
	}
	return strconv.Quote(symbol)
}
