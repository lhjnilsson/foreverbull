package command

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/repository"
	"github.com/lhjnilsson/foreverbull/pkg/finance"
	"github.com/lhjnilsson/foreverbull/pkg/pb"
	"github.com/lhjnilsson/foreverbull/pkg/pb/backtest"
)

func isPriceOverlap(text string) bool {
	return strings.Contains(text, " has prices for both ")
}

func discardPriceOverlaps(ctx context.Context, backtests repository.Backtest, name, id string, overlaps []string) error {
	err := deleteStoredBacktest(ctx, backtests, name, id)
	removed := err == nil || errors.Is(err, pgx.ErrNoRows)
	return plainSentence(priceOverlapMessage(name, overlaps, removed))
}

func priceOverlapMessage(name string, overlaps []string, removed bool) string {
	sort.Strings(overlaps)
	lead := overlaps[0]
	if !strings.HasSuffix(lead, ".") {
		lead += "."
	}
	if len(overlaps) > 1 {
		var b strings.Builder
		for _, line := range overlaps {
			fmt.Fprintf(&b, "%s\n", strings.TrimRight(line, "."))
		}
		lead = strings.TrimRight(b.String(), "\n")
	}
	if removed {
		return fmt.Sprintf("Couldn't load data for backtest '%s': %s The backtest wasn't saved.", name, lead)
	}
	return fmt.Sprintf("Couldn't load data for backtest '%s': %s The backtest was partly saved and couldn't be removed. Run `fbull backtest delete %s` first.",
		name, lead, name)
}

func listedSymbols(b *backtest.Backtest) []string {
	seen := make(map[string]struct{})
	out := make([]string, 0)
	for _, period := range b.GetPeriods() {
		for _, asset := range period.GetAssets() {
			symbol := finance.StoredSymbol(asset.GetSymbol())
			if symbol == "" {
				continue
			}
			if _, ok := seen[symbol]; ok {
				continue
			}
			seen[symbol] = struct{}{}
			out = append(out, symbol)
		}
	}
	return out
}

func periodSpan(periods []*backtest.Backtest_Period) (time.Time, time.Time, bool) {
	var start, end time.Time
	found := false
	for _, period := range periods {
		if period.GetStartDate() == nil || period.GetEndDate() == nil {
			continue
		}
		from := pb.DateToTime(period.GetStartDate())
		to := pb.DateToTime(period.GetEndDate())
		if !found || from.Before(start) {
			start = from
		}
		if !found || to.After(end) {
			end = to
		}
		found = true
	}
	return start, end, found
}

type companyTicker struct {
	symbol string
	from   time.Time
}

// refuseSameCompany fails create, before anything is stored, when two listed
// tickers are one instrument on the backtest's dates.
func refuseSameCompany(ctx context.Context, db *pgxpool.Pool, b *backtest.Backtest) error {
	symbols := listedSymbols(b)
	if len(symbols) < 2 {
		return nil
	}
	start, end, ok := periodSpan(b.GetPeriods())
	if !ok {
		return nil
	}
	rows, err := db.Query(ctx, `
		SELECT s.isin, s.symbol, s.from_date
		  FROM asset_symbol s
		 WHERE s.symbol = ANY($1)
		   AND s.from_date <= $3::date
		   AND (s.to_date IS NULL OR s.to_date > $2::date)
		 ORDER BY s.isin, s.from_date, s.symbol`,
		symbols, start.Format(time.DateOnly), end.Format(time.DateOnly))
	if err != nil {
		return fmt.Errorf("error reading ticker names: %w", err)
	}
	defer rows.Close()
	byISIN := make(map[string][]companyTicker)
	for rows.Next() {
		var isin, symbol string
		var from time.Time
		if err := rows.Scan(&isin, &symbol, &from); err != nil {
			return fmt.Errorf("error reading ticker names: %w", err)
		}
		byISIN[isin] = append(byISIN[isin], companyTicker{symbol: symbol, from: from})
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("error reading ticker names: %w", err)
	}
	isins := make([]string, 0)
	for isin, tickers := range byISIN {
		if distinctSymbols(tickers) < 2 {
			continue
		}
		isins = append(isins, isin)
	}
	if len(isins) == 0 {
		return nil
	}
	sort.Strings(isins)
	return plainSentence(sameCompanyMessage(b.GetName(), byISIN[isins[0]]))
}

func distinctSymbols(tickers []companyTicker) int {
	seen := make(map[string]struct{}, len(tickers))
	for _, ticker := range tickers {
		seen[ticker.symbol] = struct{}{}
	}
	return len(seen)
}

func sameCompanyMessage(name string, tickers []companyTicker) string {
	sort.Slice(tickers, func(i, j int) bool {
		if tickers[i].from.Equal(tickers[j].from) {
			return tickers[i].symbol < tickers[j].symbol
		}
		return tickers[i].from.Before(tickers[j].from)
	})
	ordered := make([]string, 0, len(tickers))
	seen := make(map[string]struct{}, len(tickers))
	for _, ticker := range tickers {
		if _, ok := seen[ticker.symbol]; ok {
			continue
		}
		seen[ticker.symbol] = struct{}{}
		ordered = append(ordered, ticker.symbol)
	}
	latest := ordered[len(ordered)-1]
	earlier := strings.Join(ordered[:len(ordered)-1], ", ")
	return fmt.Sprintf("Couldn't create backtest '%s': %s are the same company (%s, earlier %s). "+
		"The backtest wasn't saved. Remove one of them, then run `fbull backtest create` again.",
		name, joinAnd(ordered), latest, earlier)
}

func joinAnd(symbols []string) string {
	switch len(symbols) {
	case 0:
		return ""
	case 1:
		return symbols[0]
	case 2:
		return symbols[0] + " and " + symbols[1]
	default:
		return strings.Join(symbols[:len(symbols)-1], ", ") + " and " + symbols[len(symbols)-1]
	}
}
