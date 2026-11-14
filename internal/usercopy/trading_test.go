package usercopy

import (
	"testing"

	"github.com/stretchr/testify/require"
)

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
