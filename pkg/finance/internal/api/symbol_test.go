package api

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// validSymbol accepts the tickers the backtest service's refused-bar pattern
// accepts, and nothing a user message should not repeat.
func TestValidSymbol(t *testing.T) {
	for _, tc := range []struct {
		symbol string
		valid  bool
	}{
		{"AAPL", true},
		{"BRK-B", true},
		{"BRK.B", true},
		{"^GSPC", true},
		{"EURUSD=X", true},
		{"ADS.DE", true},
		{"A_B", true},
		{strings.Repeat("A", 32), true},
		{"", false},
		{strings.Repeat("A", 33), false},
		{"AAPL ", false},
		{" AAPL", false},
		{"AA PL", false},
		{"AAPL\n", false},
		{"AAPL\x1b[31m", false},
		{"<script>", false},
		{"AAPL/../x", false},
		{"ÅAPL", false},
		{"https://x.example/?k=v", false},
	} {
		assert.Equal(t, tc.valid, validSymbol(tc.symbol), "%q", tc.symbol)
	}
}

func TestLoggedSymbolQuotesAndCaps(t *testing.T) {
	assert.Equal(t, `"A\nB"`, loggedSymbol("A\nB"))
	assert.Equal(t, `"`+strings.Repeat("x", maxLoggedSymbol)+`"`, loggedSymbol(strings.Repeat("x", 500)))
}
