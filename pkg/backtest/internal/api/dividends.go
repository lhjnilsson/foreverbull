package api

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/repository"
	"github.com/lhjnilsson/foreverbull/pkg/pb"
	"github.com/lhjnilsson/foreverbull/pkg/pb/backtest"
	servicePb "github.com/lhjnilsson/foreverbull/pkg/pb/backtest/service"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ListDividends is the stored dividends of one backtest: its provider, its
// enabled symbols and its benchmark, with an ex-date inside its date span.
// A missing backtest is not found. A backtest with no span or no symbols
// has an empty list. The pay date is stored as sent, or nil.
func (bs *BacktestServer) ListDividends(ctx context.Context, name, owner string) ([]repository.StoredDividend, error) {
	resp, err := bs.GetBacktest(ctx, &servicePb.GetBacktestRequest{Name: name, Owner: owner})
	if err != nil {
		return nil, err
	}
	item := resp.GetBacktest()
	if item == nil {
		return nil, status.Error(codes.NotFound, "not found")
	}
	symbols, start, end, ok := dividendWindow(item)
	if !ok {
		return []repository.StoredDividend{}, nil
	}
	if item.GetProvider() == "" {
		return nil, fmt.Errorf("backtest %s has no provider", item.GetName())
	}
	rows, err := (&repository.Backtest{Conn: bs.pgx}).ListDividends(ctx, item.GetProvider(), symbols, start, end)
	if err != nil {
		return nil, err
	}
	return rows, nil
}

func dividendWindow(item *backtest.Backtest) (symbols []string, start, end time.Time, ok bool) {
	if item == nil {
		return nil, time.Time{}, time.Time{}, false
	}
	seen := make(map[string]struct{})
	add := func(symbol string) {
		symbol = strings.TrimSpace(symbol)
		if symbol == "" {
			return
		}
		seen[symbol] = struct{}{}
	}
	add(item.GetBenchmark())
	var opened bool
	for _, period := range item.GetPeriods() {
		if period.GetStartDate() == nil || period.GetEndDate() == nil {
			continue
		}
		periodStart := pb.DateToTime(period.GetStartDate())
		periodEnd := pb.DateToTime(period.GetEndDate())
		if !opened || periodStart.Before(start) {
			start = periodStart
		}
		if !opened || periodEnd.After(end) {
			end = periodEnd
		}
		opened = true
		for _, asset := range period.GetAssets() {
			if asset.GetEnabled() {
				add(asset.GetSymbol())
			}
		}
	}
	if !opened || len(seen) == 0 {
		return nil, time.Time{}, time.Time{}, false
	}
	symbols = make([]string, 0, len(seen))
	for symbol := range seen {
		symbols = append(symbols, symbol)
	}
	sort.Strings(symbols)
	return symbols, start, end, true
}
