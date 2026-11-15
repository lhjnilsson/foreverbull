package usercopy

import "testing"

func TestCloseOnUniverseDropFixed(t *testing.T) {
	const want = "Close on universe drop is set when the backtest is created and can't be changed. " +
		"To use a different setting, create a new backtest with `fbull backtest create`."
	if CloseOnUniverseDropFixed != want {
		t.Fatalf("got %q", CloseOnUniverseDropFixed)
	}
}

func TestBacktestProviderMissing(t *testing.T) {
	const want = "This backtest has no price provider, so it can't be created. " +
		"Set the provider to yahoo, stooq, or alpaca, then create the backtest again."
	if BacktestProviderMissing != want {
		t.Fatalf("got %q", BacktestProviderMissing)
	}
}
