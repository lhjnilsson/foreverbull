package marketdata

import (
	"fmt"
	"strings"
	"time"
)

// MissingPriceError is returned by GetOHLC when the provider sent a session
// with no price anywhere in the range asked for. Storing the others would
// leave a gap or a short span, so the symbol's download fails instead and
// the backtest create names the days (pkg/backtest/universe.md §7.5).
type MissingPriceError struct {
	Provider string
	Symbol   string
	// Days are the sessions with no price, oldest first, as UTC dates.
	Days []time.Time
}

func (e *MissingPriceError) Error() string {
	return fmt.Sprintf("%s sent no price for %s on %s", e.Provider, e.Symbol, MissingPriceDays(e.Days))
}

// MissingPriceDays writes days as comma-separated dates.
func MissingPriceDays(days []time.Time) string {
	shown := make([]string, 0, len(days))
	for _, day := range days {
		shown = append(shown, day.UTC().Format(time.DateOnly))
	}
	return strings.Join(shown, ", ")
}
