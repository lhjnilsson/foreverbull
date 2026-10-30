package command

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

const stooqExchangeText = "stooq doesn't support the .TO exchange (SHOP.TO). " +
	"Supported suffixes: .DE, .L, .US. Try another provider with --provider."

// Only the finance service's exact exchange refusal for this symbol gives
// the lead; create writes its own advice after it.
func TestStooqExchangeLeadOnlyMatchesTheRefusal(t *testing.T) {
	lead, ok := stooqExchangeLead(stooqExchangeText, "SHOP.TO")
	require.True(t, ok)
	require.Equal(t, "stooq doesn't support the .TO exchange (SHOP.TO). Supported suffixes: .DE, .L, .US.", lead)

	for name, tc := range map[string]struct{ text, symbol string }{
		"other symbol":      {stooqExchangeText, "RY.TO"},
		"suffix mismatch":   {"stooq doesn't support the .V exchange (SHOP.TO). Supported suffixes: .DE, .L, .US. Try another provider with --provider.", "SHOP.TO"},
		"trailing raw text": {stooqExchangeText + " <b>x</b>", "SHOP.TO"},
		"leading raw text":  {"error: " + stooqExchangeText, "SHOP.TO"},
		"no advice":         {"stooq doesn't support the .TO exchange (SHOP.TO). Supported suffixes: .DE, .L, .US.", "SHOP.TO"},
		"empty":             {"", "SHOP.TO"},
	} {
		_, ok := stooqExchangeLead(tc.text, tc.symbol)
		require.False(t, ok, name)
	}
}

func stooqInvalidTickerText(position, total string) string {
	return fmt.Sprintf("stooq can't use symbol %s of %s because it isn't a valid ticker. "+
		"Check the spelling, for example VOD.L or AAPL.", position, total)
}

// Only the exact invalid-ticker refusal, for the count this create sent and
// the symbol at that position, gives a position.
func TestStooqInvalidTickerPositionOnlyMatchesTheRefusal(t *testing.T) {
	sent := make([]string, 99)
	for i := range sent {
		sent[i] = fmt.Sprintf("S%02d.L", i+1)
	}
	for _, position := range []int{1, 7, 99} {
		got, ok := stooqInvalidTickerPosition(stooqInvalidTickerText(fmt.Sprint(position), "99"), sent[position-1], sent)
		require.True(t, ok, position)
		require.Equal(t, position, got)
	}

	for name, tc := range map[string]struct{ text, symbol string }{
		"count not sent":    {stooqInvalidTickerText("7", "98"), sent[6]},
		"position past end": {stooqInvalidTickerText("100", "99"), sent[6]},
		"position zero":     {stooqInvalidTickerText("0", "99"), sent[6]},
		"leading zero":      {stooqInvalidTickerText("07", "99"), sent[6]},
		"other symbol":      {stooqInvalidTickerText("7", "99"), sent[7]},
		"symbol echoed":     {stooqInvalidTickerText("S07.L", "99"), sent[6]},
		"trailing raw text": {stooqInvalidTickerText("7", "99") + " <b>x</b>", sent[6]},
		"old wording":       {"stooq can't use this symbol because it isn't a valid ticker. Check the spelling, for example VOD.L or AAPL.", sent[6]},
		"empty":             {"", sent[6]},
	} {
		_, ok := stooqInvalidTickerPosition(tc.text, tc.symbol, sent)
		require.False(t, ok, name)
	}
}
