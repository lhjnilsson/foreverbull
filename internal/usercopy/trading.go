package usercopy

import (
	"strings"
	"unicode"
)

// TradingProviderMissing is the sentence when a trading session has no price
// provider. backtest is the backtest name when the caller has it. An empty
// name leaves out the delete step.
func TradingProviderMissing(backtest string) string {
	return tradingProviderSentence(
		"This backtest has no price provider, so trading can't load prices.",
		backtest,
		"Create the backtest again with --provider yahoo, stooq, or alpaca, then run `fbull trading run` again.",
	)
}

// TradingProviderUnknown is the sentence when that provider is not one of
// yahoo, stooq, or alpaca. provider is the value that was set. backtest is
// the backtest name when the caller has it.
func TradingProviderUnknown(provider, backtest string) string {
	shown := shownSetting(provider)
	lead := "Trading can't use price provider " + shown + ". It must be yahoo, stooq, or alpaca."
	if shown == "that value" {
		lead = "Trading can't use that price provider. It must be yahoo, stooq, or alpaca."
	}
	return tradingProviderSentence(
		lead,
		backtest,
		"Create the backtest again with --provider set to one of those, then run `fbull trading run` again.",
	)
}

func tradingProviderSentence(lead, backtest, create string) string {
	name := strings.TrimSpace(backtest)
	if name == "" {
		return lead + " " + create
	}
	return lead + " Run `fbull backtest delete " + name + "` first. " + create
}

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
