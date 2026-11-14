package usercopy

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTradingProviderSentences(t *testing.T) {
	t.Parallel()
	require.Equal(t,
		"This backtest has no price provider, so trading can't load prices. Create the backtest again with --provider yahoo, stooq, or alpaca, then run `fbull trading run` again.",
		TradingProviderMissing(""))
	require.Equal(t,
		"This backtest has no price provider, so trading can't load prices. Run `fbull backtest delete spy` first. Create the backtest again with --provider yahoo, stooq, or alpaca, then run `fbull trading run` again.",
		TradingProviderMissing("spy"))
	require.Equal(t,
		"Trading can't use price provider 'binance'. It must be yahoo, stooq, or alpaca. Create the backtest again with --provider set to one of those, then run `fbull trading run` again.",
		TradingProviderUnknown("binance", ""))
	require.Equal(t,
		"Trading can't use price provider 'binance'. It must be yahoo, stooq, or alpaca. Run `fbull backtest delete spy` first. Create the backtest again with --provider set to one of those, then run `fbull trading run` again.",
		TradingProviderUnknown("binance", "spy"))
	require.Equal(t,
		"Trading can't use that price provider. It must be yahoo, stooq, or alpaca. Create the backtest again with --provider set to one of those, then run `fbull trading run` again.",
		TradingProviderUnknown("yahoo\nDROP", ""))
	require.Equal(t,
		"Trading can't use that price provider. It must be yahoo, stooq, or alpaca. Run `fbull backtest delete spy` first. Create the backtest again with --provider set to one of those, then run `fbull trading run` again.",
		TradingProviderUnknown("yahoo\nDROP", "spy"))
	require.Equal(t,
		"Trading can't use that price provider. It must be yahoo, stooq, or alpaca. Create the backtest again with --provider set to one of those, then run `fbull trading run` again.",
		TradingProviderUnknown("alpaca\x7f", ""))
}

func TestStooqExchangeSentences(t *testing.T) {
	t.Parallel()
	require.Equal(t, "Stooq needs an exchange, but STOOQ_EXCHANGE isn't set on the broker. Set the Helm value broker.stooqExchange to US and upgrade the release, then run `fbull backtest create` or `fbull trading run` again. With US, foreverbull asks Stooq for symbols such as aapl.us.", StooqExchangeMissing)
	require.Equal(t,
		"Stooq doesn't support exchange 'L' yet. Only US is supported. Set the Helm value broker.stooqExchange to US and upgrade the release, then run `fbull backtest create` or `fbull trading run` again.",
		StooqExchangeUnsupported("L"))
	require.Equal(t,
		"Stooq doesn't support exchange 'DE' yet. Only US is supported. Set the Helm value broker.stooqExchange to US and upgrade the release, then run `fbull backtest create` or `fbull trading run` again.",
		StooqExchangeUnsupported("DE"))
	require.Equal(t,
		"Stooq doesn't support exchange 'UK' yet. Only US is supported. Set the Helm value broker.stooqExchange to US and upgrade the release, then run `fbull backtest create` or `fbull trading run` again.",
		StooqExchangeUnsupported("UK"))
	require.Equal(t,
		"Stooq doesn't support exchange 'TO' yet. Only US is supported. Set the Helm value broker.stooqExchange to US and upgrade the release, then run `fbull backtest create` or `fbull trading run` again.",
		StooqExchangeUnsupported("TO"))
	require.Equal(t,
		"Stooq doesn't support that exchange yet. Only US is supported. Set the Helm value broker.stooqExchange to US and upgrade the release, then run `fbull backtest create` or `fbull trading run` again.",
		StooqExchangeUnsupported("x\x1b[2J"))
	require.Equal(t,
		"Stooq doesn't support that exchange yet. Only US is supported. Set the Helm value broker.stooqExchange to US and upgrade the release, then run `fbull backtest create` or `fbull trading run` again.",
		StooqExchangeUnsupported("US\x7f"))
}
