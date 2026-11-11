package usercopy

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// SnapshotCacheReadFailed is the sentence for an ETF holdings cache folder
// that couldn't be read. The folder is inside the broker, not on the
// user's disk, so the step is a report.
func SnapshotCacheReadFailed(dir, cause string) string {
	return fmt.Sprintf("Couldn't read the saved ETF holdings in %s: %s. %s", dir, cause, ReportIt)
}

// SnapshotCacheWriteFailed is the sentence for downloaded ETF holdings that
// couldn't be saved to the cache folder.
func SnapshotCacheWriteFailed(dir, cause string) string {
	return fmt.Sprintf("Couldn't save the downloaded ETF holdings in %s: %s. %s", dir, cause, ReportIt)
}

// ETFNoHoldingsAtStart is the sentence when no holdings list was published
// on or before the backtest start. first is the earliest list after it.
// The exact string is pinned in snapshot_test.go.
func ETFNoHoldingsAtStart(etf string, start, first time.Time) string {
	startText := calendarDate(start)
	firstText := calendarDate(first)
	return fmt.Sprintf("%s has no holdings list on or before %s. The first list was published on %s. Choose a start on or after %s.",
		etf, startText, firstText, firstText)
}

// ETFNoHoldingsInWindow is the sentence when no holdings list was published
// anywhere from start through end. The exact string is pinned in snapshot_test.go.
func ETFNoHoldingsInWindow(etf string, start, end time.Time) string {
	return fmt.Sprintf("%s has no holdings list between %s and %s. Choose dates that include a published list.",
		etf, calendarDate(start), calendarDate(end))
}

// ETFHoldingsGap is the sentence when sampled months inside the backtest
// window have no holdings list. missing is those as-of dates; at least one.
// Months in a row with no list are one gap ("2020-01-31 to 2020-02-29").
// Separate months are listed. The exact strings are pinned in snapshot_test.go.
func ETFHoldingsGap(etf string, missing []time.Time) string {
	if len(missing) == 0 {
		panic("ETFHoldingsGap called with no missing month")
	}
	listed := joinHoldingsDates(missing, "and")
	avoid := joinHoldingsDates(missing, "or")
	return fmt.Sprintf("%s has no holdings list for %s. Choose dates that don't include %s.", etf, listed, avoid)
}

type holdingsRun struct {
	from time.Time
	to   time.Time
}

// holdingsAdjacent is true when next is the same month as prev or the next
// month, so successive unpublished month-ends are one gap and a published
// month between them keeps them apart.
func holdingsAdjacent(prev, next time.Time) bool {
	prev = time.Date(prev.Year(), prev.Month(), prev.Day(), 0, 0, 0, 0, time.UTC)
	next = time.Date(next.Year(), next.Month(), next.Day(), 0, 0, 0, 0, time.UTC)
	limit := time.Date(prev.Year(), prev.Month()+2, 0, 0, 0, 0, 0, time.UTC)
	return !next.After(limit)
}

func holdingsRuns(missing []time.Time) []holdingsRun {
	dates := make([]time.Time, len(missing))
	for i, day := range missing {
		dates[i] = time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, time.UTC)
	}
	// dates is a copy; sort it so the sentence does not depend on caller order.
	sort.Slice(dates, func(i, j int) bool { return dates[i].Before(dates[j]) })
	runs := []holdingsRun{{from: dates[0], to: dates[0]}}
	for _, day := range dates[1:] {
		last := &runs[len(runs)-1]
		if holdingsAdjacent(last.to, day) {
			last.to = day
			continue
		}
		runs = append(runs, holdingsRun{from: day, to: day})
	}
	return runs
}

func formatHoldingsRun(run holdingsRun) string {
	from := calendarDate(run.from)
	if run.from.Equal(run.to) {
		return from
	}
	return from + " to " + calendarDate(run.to)
}

func joinHoldingsDates(missing []time.Time, lastSep string) string {
	runs := holdingsRuns(missing)
	parts := make([]string, len(runs))
	for i, run := range runs {
		parts[i] = formatHoldingsRun(run)
	}
	if len(parts) < 2 {
		return strings.Join(parts, "")
	}
	return strings.Join(parts[:len(parts)-1], ", ") + " " + lastSep + " " + parts[len(parts)-1]
}

func calendarDate(day time.Time) string {
	return time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, time.UTC).Format(time.DateOnly)
}
