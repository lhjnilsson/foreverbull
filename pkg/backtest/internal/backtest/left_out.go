package backtest

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/repository"
	"github.com/lhjnilsson/foreverbull/pkg/finance/supplier"
	commonPB "github.com/lhjnilsson/foreverbull/pkg/pb"
	pb "github.com/lhjnilsson/foreverbull/pkg/pb/backtest"
)

// lastBarsByName is the membership lookup that also reads each member's last
// stored bar. repository.UniverseMember implements it.
type lastBarsByName interface {
	LastBars(ctx context.Context, backtestName, provider string, end time.Time) ([]repository.AliasLastBar, error)
}

type lastBarsByID interface {
	LastBarsByID(ctx context.Context, backtestID, provider string, end time.Time) ([]repository.AliasLastBar, error)
}

// leftOut records the members a run passed over because their prices had
// ended: an ETF member that was delisted keeps its membership until the next
// month-end holdings list drops it, but it has no bar to trade on. Each
// symbol keeps the date of its last bar. The runner goroutine writes it and
// RunNotices reads it after the period channel has closed.
type leftOut struct {
	lastBar map[string]time.Time
}

func newLeftOut() *leftOut {
	return &leftOut{lastBar: make(map[string]time.Time)}
}

// barsEnd is the exclusive upper bound on the bars the bundle holds: the day
// after the backtest's last period. A later download for another backtest
// can store bars past it, and those are not in this run's bundle.
func barsEnd(stored *pb.Backtest) time.Time {
	periods := stored.GetPeriods()
	if len(periods) > 0 && periods[len(periods)-1].GetEndDate() != nil {
		return commonPB.DateToTime(periods[len(periods)-1].GetEndDate()).AddDate(0, 0, 1)
	}
	return time.Date(9999, 1, 1, 0, 0, 0, 0, time.UTC)
}

func barsProvider(stored *pb.Backtest) string {
	provider := strings.TrimSpace(stored.GetProvider())
	if provider == "" {
		return supplier.DefaultProvider
	}
	return provider
}

// withoutEndedPrices wraps a member source so a ticker whose ISIN has no bar
// on or after the UTC date of t is not passed to Process. A member with no
// bar at all is left as it is: the ingest refuses such a backtest, so it
// cannot reach a run.
func withoutEndedPrices(source symbolSource, aliases []repository.AliasLastBar, left *leftOut) symbolSource {
	return func(ctx context.Context, t time.Time) ([]string, error) {
		symbols, err := source(ctx, t)
		if err != nil {
			return nil, err
		}
		day := dayOf(t)
		kept := make([]string, 0, len(symbols))
		for _, symbol := range symbols {
			last, ok := lastBarAt(aliases, symbol, day)
			if ok && last.Before(day) {
				if previous, seen := left.lastBar[symbol]; !seen || last.After(previous) {
					left.lastBar[symbol] = last
				}
				continue
			}
			kept = append(kept, symbol)
		}
		return kept, nil
	}
}

// lastBarAt returns the UTC date of the last bar of the ISIN that holds
// symbol on day. ok is false when no alias of symbol covers day or the ISIN
// has no bar.
func lastBarAt(aliases []repository.AliasLastBar, symbol string, day time.Time) (time.Time, bool) {
	for _, alias := range aliases {
		if alias.Symbol != symbol || alias.LastBar == nil {
			continue
		}
		if day.Before(dayOf(alias.FromDate)) {
			continue
		}
		if alias.ToDate != nil && !day.Before(dayOf(*alias.ToDate)) {
			continue
		}
		return dayOf(*alias.LastBar), true
	}
	return time.Time{}, false
}

func dayOf(t time.Time) time.Time {
	year, month, day := t.UTC().Date()
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}

// notices is one line per last-bar date, oldest first, with the symbols
// sorted: "3 symbols had no prices after 2026-03-31 and were left out: ABC, DEF, GHI."
func (l *leftOut) notices() []string {
	if l == nil || len(l.lastBar) == 0 {
		return nil
	}
	byDate := make(map[time.Time][]string)
	for symbol, last := range l.lastBar {
		byDate[last] = append(byDate[last], symbol)
	}
	dates := make([]time.Time, 0, len(byDate))
	for day := range byDate {
		dates = append(dates, day)
	}
	slices.SortFunc(dates, func(a, b time.Time) int { return a.Compare(b) })
	lines := make([]string, 0, len(dates))
	for _, day := range dates {
		lines = append(lines, leftOutLine(day, byDate[day]))
	}
	return lines
}

func leftOutLine(last time.Time, symbols []string) string {
	slices.Sort(symbols)
	names := strings.Join(symbols, ", ")
	if len(symbols) == 1 {
		return fmt.Sprintf("1 symbol had no prices after %s and was left out: %s.", last.Format(time.DateOnly), names)
	}
	return fmt.Sprintf("%d symbols had no prices after %s and were left out: %s.", len(symbols), last.Format(time.DateOnly), names)
}
