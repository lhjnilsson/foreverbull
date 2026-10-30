package ticker

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Valid accepts the tickers the backtest service's refused-bar pattern
// accepts, and nothing a user message should not repeat.
func TestValid(t *testing.T) {
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
		assert.Equal(t, tc.valid, Valid(tc.symbol), "%q", tc.symbol)
	}
}

func TestLoggedQuotesAndCaps(t *testing.T) {
	assert.Equal(t, `"A\nB"`, Logged("A\nB"))
	assert.Equal(t, `"`+strings.Repeat("x", maxLogged)+`"`, Logged(strings.Repeat("x", 500)))
}

func TestInvalidMessage(t *testing.T) {
	assert.Equal(t, "symbol 7 of 99 isn't a valid ticker (use 1 to 32 letters, digits, or . ^ = _ -)", InvalidMessage(7, 99))
}
