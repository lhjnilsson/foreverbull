// Package ticker is the symbol shape the finance service accepts, stores bars
// under, and names in a user message, for both the download RPC and ingest.
package ticker

import (
	"fmt"
	"regexp"
	"strconv"
)

// pattern is the ticker shape a download or ingest accepts, stores bars
// under, and names in a client message. The backtest service shows a refused-bar
// sentence only when its symbol has this shape, so a provider or caller
// string outside it (spaces, control bytes, markup, 33+ bytes) is refused
// here instead of reaching a user.
var pattern = regexp.MustCompile(`^[A-Za-z0-9.^=_-]{1,32}$`)

// Valid reports whether symbol has the ticker shape.
func Valid(symbol string) bool {
	return pattern.MatchString(symbol)
}

// InvalidMessage refuses a request whose symbol at the 1-based position is
// outside the pattern. It names the position and the request's
// symbol count, never the symbol, because that is the text that failed the
// check. The backtest service matches this exact shape to build its sentence.
func InvalidMessage(position, total int) string {
	return fmt.Sprintf("symbol %d of %d isn't a valid ticker (use 1 to 32 letters, digits, or . ^ = _ -)", position, total)
}

// maxLogged caps how much of a refused symbol goes into a log line.
const maxLogged = 64

// Logged quotes a refused symbol for a log field, so control bytes stay
// escaped and a long provider string can't flood the log.
func Logged(symbol string) string {
	if len(symbol) > maxLogged {
		symbol = symbol[:maxLogged]
	}
	return strconv.Quote(symbol)
}
