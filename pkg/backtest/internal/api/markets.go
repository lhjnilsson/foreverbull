package api

import (
	"context"
	"errors"
	"strings"

	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/internal/usercopy"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/repository"
	"github.com/lhjnilsson/foreverbull/pkg/finance"
	"github.com/lhjnilsson/foreverbull/pkg/pb/backtest"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// refuseRunMarkets stops a backtest run whose symbols are listed outside
// the US or priced in a currency other than USD. Nothing is started.
// Create and ingest store the decision. A run of a backtest that was
// already checked does not ask again. One that was not is checked once
// here: a pass stores the decision, a refusal or a failed lookup stops
// the run. Stored bars are not a decision. They are shared by every
// backtest that uses the symbol.
func (s *grpcSessionServer) refuseRunMarkets(ctx context.Context, stored *backtest.Backtest) error {
	if stored == nil {
		return nil
	}
	var backtests repository.Backtest
	if s.db != nil {
		backtests = repository.Backtest{Conn: s.db}
		checked, err := backtests.MarketsChecked(ctx, stored.GetName(), s.backtestID(ctx))
		if err != nil {
			return status.Error(codes.Internal, "could not load the backtest")
		}
		if checked {
			return nil
		}
	}
	members, err := s.runMembers(ctx, stored)
	if err != nil {
		return status.Error(codes.Internal, "could not load the backtest")
	}
	listed, bench, err := finance.RefuseListed(ctx, stored.GetProvider(), stored.GetProfile(), members, stored.GetBenchmark())
	if err != nil && !isRunLookup(err) {
		return status.Error(codes.FailedPrecondition, err.Error())
	}
	names, provider := runLookupNames(err)
	switch {
	case len(names) > 0 && (len(listed) > 0 || bench != nil):
		return status.Error(codes.FailedPrecondition, usercopy.GlobalMarketsRunMixed(stored.GetName(), listed, bench, provider, names))
	case len(names) > 0:
		return status.Error(codes.FailedPrecondition, usercopy.LookupRun(stored.GetName(), provider, names))
	case len(listed) > 0 || bench != nil:
		return status.Error(codes.FailedPrecondition, usercopy.GlobalMarketsRun(stored.GetName(), listed, bench))
	}
	// An unknown symbol is not a refusal and not a failed lookup. It also
	// did not answer, so the decision is not stored and the next run asks again.
	asked := append(append([]string{}, members...), stored.GetBenchmark())
	if !finance.ListingsAnswered(stored.GetProvider(), asked) {
		return nil
	}
	if s.db != nil {
		if err := backtests.MarkMarketsChecked(ctx, stored.GetName(), s.backtestID(ctx)); err != nil {
			return status.Error(codes.Internal, "could not load the backtest")
		}
	}
	return nil
}

func (s *grpcSessionServer) backtestID(ctx context.Context) string {
	if !environment.GetAuthEnabled() || s.session == nil {
		return ""
	}
	sessions := repository.Session{Conn: s.db}
	id, err := sessions.BacktestID(ctx, s.session.GetId())
	if err != nil {
		return ""
	}
	return id
}

func isRunLookup(err error) bool {
	var many *finance.QuoteFailures
	var one *finance.QuoteFailure
	return errors.As(err, &many) || errors.As(err, &one)
}

func runLookupNames(err error) (names []string, provider string) {
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

// runMembers are the tickers a run would ingest, without the benchmark.
// Universe aliases win when the backtest has them, the same way ingest does.
func (s *grpcSessionServer) runMembers(ctx context.Context, stored *backtest.Backtest) ([]string, error) {
	symbols := make([]string, 0)
	if stored.GetHasUniverseMembers() && s.db != nil {
		column := "m.backtest_name"
		key := stored.GetName()
		if environment.GetAuthEnabled() {
			sessions := repository.Session{Conn: s.db}
			id, err := sessions.BacktestID(ctx, s.session.GetId())
			if err != nil {
				return nil, err
			}
			column = "m.backtest_id"
			key = id
		}
		rows, err := s.db.Query(ctx, `
			SELECT DISTINCT s.symbol
			  FROM universe_member m
			  JOIN asset_symbol s ON s.isin = m.isin
			 WHERE `+column+` = $1
			 ORDER BY s.symbol`, key)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		for rows.Next() {
			var symbol string
			if err := rows.Scan(&symbol); err != nil {
				return nil, err
			}
			symbols = append(symbols, symbol)
		}
		if err := rows.Err(); err != nil {
			return nil, err
		}
	}
	if len(symbols) == 0 {
		for _, period := range stored.GetPeriods() {
			for _, asset := range period.GetAssets() {
				symbols = append(symbols, asset.GetSymbol())
			}
		}
	}
	bench := strings.ToUpper(strings.TrimSpace(stored.GetBenchmark()))
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
	return out, nil
}
