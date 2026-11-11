package backtest

import (
	"testing"
	"time"
)

func TestUniverseDropLineOneSymbol(t *testing.T) {
	got := universeDropLine(time.Date(2023, 1, 31, 0, 0, 0, 0, time.UTC), []string{"XYZ"})
	const want = "1 symbol left the universe on 2023-01-31 and a held position was closed at that session's close: XYZ."
	if got != want {
		t.Fatalf("got %q", got)
	}
}

func TestUniverseDropLineSeveralSymbols(t *testing.T) {
	got := universeDropLine(time.Date(2023, 1, 31, 0, 0, 0, 0, time.UTC), []string{"GHI", "ABC", "DEF"})
	const want = "3 symbols left the universe on 2023-01-31 and held positions were closed at that session's close: ABC, DEF, GHI."
	if got != want {
		t.Fatalf("got %q", got)
	}
}

func TestUniverseDropLineStopsAfterFiveNames(t *testing.T) {
	got := universeDropLine(time.Date(2023, 1, 31, 0, 0, 0, 0, time.UTC), []string{"G", "F", "E", "D", "C", "B", "A"})
	const want = "7 symbols left the universe on 2023-01-31 and held positions were closed at that session's close: A, B, C, D, E and 2 more."
	if got != want {
		t.Fatalf("got %q", got)
	}
}

func TestUniverseDropNoticesAreOneLinePerSessionOldestFirst(t *testing.T) {
	drops := newUniverseDrops()
	drops.add(time.Date(2023, 3, 15, 21, 0, 0, 0, time.UTC), []string{"XYZ"})
	drops.add(time.Date(2023, 1, 31, 21, 0, 0, 0, time.UTC), []string{"XYZ"})
	got := drops.notices()
	if len(got) != 2 {
		t.Fatalf("lines %v", got)
	}
	if got[0] != "1 symbol left the universe on 2023-01-31 and a held position was closed at that session's close: XYZ." {
		t.Fatalf("first %q", got[0])
	}
	if got[1] != "1 symbol left the universe on 2023-03-15 and a held position was closed at that session's close: XYZ." {
		t.Fatalf("second %q", got[1])
	}
}
