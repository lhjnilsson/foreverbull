package command

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/lhjnilsson/foreverbull/internal/usercopy"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/repository"
	"github.com/lhjnilsson/foreverbull/pkg/finance"
	"github.com/lhjnilsson/foreverbull/pkg/pb/backtest"
	"github.com/rs/zerolog/log"
)

// refuseCreateMarkets refuses a create whose symbols are listed outside the
// US or priced in a currency other than USD. Nothing has to be stored yet:
// removed is true when the backtest row is already gone. benchmarkFixed is
// set when the ETF path chose the benchmark.
func refuseCreateMarkets(ctx context.Context, name, provider, profile, benchmark string, members []string, removed, benchmarkFixed bool) error {
	listed, bench, err := finance.RefuseListed(ctx, provider, profile, members, benchmark)
	if err != nil && !isListingLookup(err) {
		return err
	}
	return createMarketSentence(name, listed, bench, err, removed, benchmarkFixed)
}

// discardRefusedMarkets deletes a backtest that already has a row and
// returns the create sentence. A delete that fails keeps the partly-saved step.
// A listing lookup that failed is deleted the same way: the row goes first.
func discardRefusedMarkets(ctx context.Context, backtests repository.Backtest, name, id, provider, profile, benchmark string, members []string) error {
	if checked, err := backtests.MarketsChecked(ctx, name, id); err != nil {
		return err
	} else if checked {
		return nil
	}
	listed, bench, err := finance.RefuseListed(ctx, provider, profile, members, benchmark)
	if err != nil && !isListingLookup(err) {
		return err
	}
	if len(listed) == 0 && bench == nil && !isListingLookup(err) {
		// An unknown symbol did not answer. The row stays unchecked so the
		// next download asks again.
		asked := append(append([]string{}, members...), benchmark)
		if !finance.ListingsAnswered(provider, asked) {
			return nil
		}
		if markErr := backtests.MarkMarketsChecked(ctx, name, id); markErr != nil {
			return markErr
		}
		return nil
	}
	delErr := deleteStoredBacktest(ctx, backtests, name, id)
	removed := delErr == nil || errors.Is(delErr, pgx.ErrNoRows)
	if !removed {
		log.Warn().Err(delErr).Str("backtest", name).Msg("error deleting backtest after a non-US symbol")
	}
	return createMarketSentence(name, listed, bench, err, removed, false)
}

func createMarketSentence(name string, listed []usercopy.MarketSymbol, bench *usercopy.MarketSymbol, err error, removed, benchmarkFixed bool) error {
	lookups, provider := lookupNames(err)
	if len(lookups) > 0 {
		logListingLookups(name, err)
	}
	switch {
	case len(lookups) > 0 && (len(listed) > 0 || bench != nil):
		return plainSentence(usercopy.GlobalMarketsCreateMixed(name, listed, bench, provider, lookups, removed, benchmarkFixed))
	case len(lookups) > 0:
		return plainSentence(usercopy.LookupCreate(name, provider, lookups, removed))
	case len(listed) == 0 && bench == nil:
		return nil
	default:
		return plainSentence(usercopy.GlobalMarketsCreate(name, listed, bench, removed, benchmarkFixed && bench != nil))
	}
}

func isListingLookup(err error) bool {
	var many *finance.QuoteFailures
	var one *finance.QuoteFailure
	return errors.As(err, &many) || errors.As(err, &one)
}

func lookupNames(err error) (names []string, provider string) {
	var many *finance.QuoteFailures
	if errors.As(err, &many) && many != nil {
		for _, item := range many.Items {
			if item == nil {
				continue
			}
			names = append(names, item.Symbol)
			if provider == "" {
				provider = item.Provider
			}
		}
		return names, provider
	}
	var one *finance.QuoteFailure
	if errors.As(err, &one) && one != nil {
		return []string{one.Symbol}, one.Provider
	}
	return nil, ""
}

func logListingLookups(name string, err error) {
	var many *finance.QuoteFailures
	if !errors.As(err, &many) || many == nil {
		return
	}
	for _, item := range many.Items {
		if item == nil {
			continue
		}
		log.Error().Err(item.Err).Str("backtest", name).Str("symbol", item.Symbol).Str("provider", item.Provider).
			Msg("listing lookup failed")
	}
}

// backtestMembers are the period symbols, without the benchmark and without
// duplicates. The benchmark is returned trimmed.
func backtestMembers(b *backtest.Backtest) (members []string, benchmark string) {
	return memberSymbols(periodSymbols(b), b.GetBenchmark()), strings.TrimSpace(b.GetBenchmark())
}

// memberSymbols drops the benchmark and repeated tickers. Comparison is
// case-insensitive; the first spelling is kept.
func memberSymbols(symbols []string, benchmark string) []string {
	bench := strings.ToUpper(strings.TrimSpace(benchmark))
	seen := map[string]struct{}{}
	if bench != "" {
		seen[bench] = struct{}{}
	}
	out := make([]string, 0, len(symbols))
	for _, symbol := range symbols {
		symbol = strings.TrimSpace(symbol)
		if symbol == "" {
			continue
		}
		key := strings.ToUpper(symbol)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, symbol)
	}
	return out
}

func periodSymbols(b *backtest.Backtest) []string {
	if b == nil {
		return nil
	}
	out := make([]string, 0)
	for _, period := range b.GetPeriods() {
		for _, asset := range period.GetAssets() {
			out = append(out, asset.GetSymbol())
		}
	}
	return out
}
