package usercopy_test

import (
	"testing"

	"github.com/lhjnilsson/foreverbull/internal/usercopy"
	"github.com/stretchr/testify/require"
)

func TestBacktestCleanupInternal(t *testing.T) {
	pricesKept := "The backtest was partly saved and couldn't be removed, and some of its stored prices couldn't be removed either."
	require.Equal(t, "Couldn't load data for backtest 'oef-2024q1': foreverbull hit an internal error while cleaning up. "+
		"The backtest was partly saved and couldn't be removed, and some of its stored prices couldn't be removed either. "+
		"Run `fbull backtest delete oef-2024q1` first, then run `fbull backtest create` again. "+
		"Please report it at https://github.com/lhjnilsson/foreverbull/issues with this message.",
		usercopy.BacktestCleanupInternal("oef-2024q1", pricesKept))
}
