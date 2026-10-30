package command

import (
	"context"
	"errors"
	"fmt"
	"regexp"

	"github.com/jackc/pgx/v5"
	"github.com/lhjnilsson/foreverbull/internal/numfmt"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/repository"
	"github.com/lhjnilsson/foreverbull/pkg/finance"
	"github.com/lhjnilsson/foreverbull/pkg/pb/backtest"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// The price download for a create is one request, and the finance service
// refuses one with more than finance.MaxDownloadSymbols distinct symbols. The
// broker counts the same way (finance.DownloadSymbolCount: distinct
// finance.StoredSymbol names, so BRK.B, BRK-B and BRKB are one) before it
// stores the backtest, so an oversized create stores
// nothing and never reaches a provider. The count it shows is that count, and
// the limit is the finance constant. The finance service's own refusal is
// only a backstop for other callers; if a create ever gets it, the create
// still shows this sentence with the broker's count.

// fileBacktestSymbols is the list a create from a backtest file downloads:
// the benchmark, then every period's assets (ingestSymbols without a stored
// universe).
func fileBacktestSymbols(b *backtest.Backtest) []string {
	symbols := []string{b.GetBenchmark()}
	for _, period := range b.GetPeriods() {
		for _, asset := range period.GetAssets() {
			symbols = append(symbols, asset.GetSymbol())
		}
	}
	return symbols
}

// tooManySymbols returns the create sentence when symbols is over the
// download cap, or nil. It runs before the backtest is stored.
func tooManySymbols(name string, symbols []string) error {
	count := finance.DownloadSymbolCount(symbols)
	if count <= finance.MaxDownloadSymbols {
		return nil
	}
	return plainSentence(fmt.Sprintf("Couldn't load data for backtest '%s': %s The backtest wasn't saved. "+
		"Remove symbols or split them across several backtests, then run `fbull backtest create` again.",
		name, tooManySymbolsLead(count)))
}

// tooManySymbolsError matches the finance service's InvalidArgument refusal of
// a request over the cap. It carries no count; the broker's count is shown.
var tooManySymbolsError = regexp.MustCompile(`^one download can carry at most [0-9,]+ distinct symbols, and this one has more\. Split it into smaller downloads\.$`)

// overCapRefusal reports whether err is the finance service's over-cap
// refusal.
func overCapRefusal(err error) bool {
	st, ok := status.FromError(err)
	return ok && st.Code() == codes.InvalidArgument && tooManySymbolsError.MatchString(st.Message())
}

// discardOversizedBacktest is the download step's own check, for a stored
// universe that grew past the cap after create counted it (an ETF member's
// stored aliases), or a refusal from the finance service. It deletes the
// backtest and shows the broker's count.
func discardOversizedBacktest(ctx context.Context, backtests repository.Backtest, name, id string, count int) error {
	err := deleteStoredBacktest(ctx, backtests, name, id)
	if err == nil || errors.Is(err, pgx.ErrNoRows) {
		return plainSentence(fmt.Sprintf("Couldn't load data for backtest '%s': %s The backtest wasn't saved. "+
			"Remove symbols or split them across several backtests, then run `fbull backtest create` again.",
			name, tooManySymbolsLead(count)))
	}
	log.Warn().Err(err).Str("backtest", name).Msg("error deleting backtest over the symbol limit")
	return plainSentence(fmt.Sprintf("Couldn't load data for backtest '%s': %s The backtest couldn't be removed, "+
		"so run `fbull backtest delete %s` before you run `fbull backtest create` again.",
		name, tooManySymbolsLead(count), name))
}

func tooManySymbolsLead(count int) string {
	return fmt.Sprintf("it has %s symbols, and the limit is %s.",
		numfmt.Thousands(count), numfmt.Thousands(finance.MaxDownloadSymbols))
}
