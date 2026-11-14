package usercopy

import (
	"strings"
	"unicode"
)

// StooqExchangeMissing is the sentence when Stooq is used without an exchange.
// US is the only exchange Stooq is asked with, and it still requests the same
// symbols, such as aapl.us.
const StooqExchangeMissing = "Stooq needs an exchange, but STOOQ_EXCHANGE isn't set on the broker. Set the Helm value broker.stooqExchange to US and upgrade the release, then run `fbull backtest create` or `fbull trading run` again. With US, foreverbull asks Stooq for symbols such as aapl.us."

// StooqExchangeUnsupported is the sentence when STOOQ_EXCHANGE or a Stooq
// client is given an exchange other than US. exchange is the value that was set.
func StooqExchangeUnsupported(exchange string) string {
	rest := " Only US is supported. Set the Helm value broker.stooqExchange to US and upgrade the release, then run `fbull backtest create` or `fbull trading run` again."
	shown := shownSetting(exchange)
	if shown == "that value" {
		return "Stooq doesn't support that exchange yet." + rest
	}
	return "Stooq doesn't support exchange " + shown + " yet." + rest
}

// shownSetting quotes a short setting value. A blank, long, or non-printing
// value is not echoed.
func shownSetting(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" || len(raw) > 32 || strings.ContainsFunc(raw, func(r rune) bool {
		return r < 0x20 || r >= unicode.MaxASCII || r == '\'' || r == '`'
	}) {
		return "that value"
	}
	return "'" + raw + "'"
}
