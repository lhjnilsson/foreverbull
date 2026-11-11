package usercopy

import "testing"

func TestCloseOnUniverseDropFixed(t *testing.T) {
	const want = "Close on universe drop is set when the backtest is created and can't be changed. " +
		"To use a different setting, create a new backtest with `fbull backtest create`."
	if CloseOnUniverseDropFixed != want {
		t.Fatalf("got %q", CloseOnUniverseDropFixed)
	}
}
