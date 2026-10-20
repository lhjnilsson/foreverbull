package api

import (
	"context"
	"fmt"
	"time"

	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/repository"
	"github.com/lhjnilsson/foreverbull/pkg/pb"
	"github.com/lhjnilsson/foreverbull/pkg/pb/backtest"
)

// memberIntervals lists a backtest's universe_member rows.
type memberIntervals interface {
	List(ctx context.Context, backtestName string) ([]repository.MemberInterval, error)
}

// executionUniverse says where the run's symbols came from and whether an
// ETF-built universe had no member on any date of the run. The session picks
// the same way (universe.md section 7.3): any universe_member row means ETF.
// Period assets are the ingest union, not membership, so they are not read.
func executionUniverse(
	ctx context.Context,
	members memberIntervals,
	execution *backtest.Execution,
) (backtest.UniverseSource, bool, error) {
	intervals, err := members.List(ctx, execution.GetBacktest())
	if err != nil {
		return backtest.UniverseSource_UNIVERSE_SOURCE_UNSPECIFIED, false,
			fmt.Errorf("error listing universe members of %s: %w", execution.GetBacktest(), err)
	}
	if len(intervals) == 0 {
		return backtest.UniverseSource_UNIVERSE_SOURCE_SYMBOL_LIST, false, nil
	}
	// Without both dates the run period is unknown, so emptiness is not claimed.
	if execution.GetStartDate() == nil || execution.GetEndDate() == nil {
		return backtest.UniverseSource_UNIVERSE_SOURCE_ETF, false, nil
	}
	start := pb.DateToTime(execution.GetStartDate())
	end := pb.DateToTime(execution.GetEndDate())
	return backtest.UniverseSource_UNIVERSE_SOURCE_ETF, universeEmptyBetween(intervals, start, end), nil
}

// universeEmptyBetween reports whether no half-open [from, to) interval covers
// any date of the closed range [start, end]. A range whose end is before its
// start has no dates, and it is not reported as empty: that is a bad run
// period, not an empty universe.
func universeEmptyBetween(intervals []repository.MemberInterval, start, end time.Time) bool {
	first := utcDate(start)
	last := utcDate(end)
	if last.Before(first) {
		return false
	}
	for _, interval := range intervals {
		if intervalCovers(interval, first, last) {
			return false
		}
	}
	return true
}

// intervalCovers reports whether [from, to) holds a date in [first, last].
// The earliest candidate is the later of from and first; it must be on or
// before last and before to.
func intervalCovers(interval repository.MemberInterval, first, last time.Time) bool {
	candidate := utcDate(interval.FromDate)
	if candidate.Before(first) {
		candidate = first
	}
	if candidate.After(last) {
		return false
	}
	if interval.ToDate == nil {
		return true
	}
	return candidate.Before(utcDate(*interval.ToDate))
}

func utcDate(t time.Time) time.Time {
	year, month, day := t.Date()
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}
