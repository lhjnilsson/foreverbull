package usercopy

import (
	"testing"
	"time"
)

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

func TestBacktestStartsBeforeCalendar(t *testing.T) {
	start := time.Date(1989, 12, 31, 0, 0, 0, 0, time.UTC)
	first := time.Date(1990, 1, 2, 0, 0, 0, 0, time.UTC)
	const want = "This backtest starts on 1989-12-31, before the first trading day in foreverbull's market calendar (1990-01-02), so it can't be created. Set the start to 1990-01-02 or later, then create the backtest again."
	if got := BacktestStartsBeforeCalendar(start, first); got != want {
		t.Fatalf("got %q", got)
	}
	if !IsBacktestStartsBeforeCalendar(want) {
		t.Fatal("the sentence was not recognized")
	}
	if IsBacktestStartsBeforeCalendar(want + " more") {
		t.Fatal("trailing text was recognized")
	}
}
