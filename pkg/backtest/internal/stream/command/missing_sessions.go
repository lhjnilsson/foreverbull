package command

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/stream"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/engine"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/repository"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/stream/dependency"
	"github.com/lhjnilsson/foreverbull/pkg/finance"
	"github.com/rs/zerolog/log"
)

// symbolGaps is one request symbol's missing sessions, as indexes into the
// period's sessions, oldest first.
type symbolGaps struct {
	symbol   string
	sessions []int
}

// missingSessions lists, per symbol in the order given, the sessions with no
// bar inside the symbol's span. The span runs from the symbol's first stored
// bar to its last over all time; sessions are the period's, so it is clipped
// to the period. Days before the first bar (a listing) or after the last (a
// delisting, which auto-closes) are not missing, and a day that is not a
// session (a holiday) never is. bars is keyed by the stored symbol, so the
// request symbol is what the user sees.
func missingSessions(sessions []time.Time, symbols []string, bars map[string]*repository.SymbolBars) []symbolGaps {
	gaps := make([]symbolGaps, 0)
	for _, symbol := range symbols {
		stored := bars[finance.StoredSymbol(symbol)]
		if stored == nil {
			continue
		}
		have := make(map[time.Time]bool, len(stored.Days))
		for _, day := range stored.Days {
			have[day] = true
		}
		var missing []int
		for i, session := range sessions {
			if session.Before(stored.First) || session.After(stored.Last) || have[session] {
				continue
			}
			missing = append(missing, i)
		}
		if len(missing) > 0 {
			gaps = append(gaps, symbolGaps{symbol: symbol, sessions: missing})
		}
	}
	return gaps
}

// sessionRanges writes missing sessions as dates, and a run of sessions in a
// row as "A to B". A weekend or holiday inside a run does not break it.
func sessionRanges(sessions []time.Time, missing []int) string {
	parts := make([]string, 0, len(missing))
	for i := 0; i < len(missing); {
		j := i
		for j+1 < len(missing) && missing[j+1] == missing[j]+1 {
			j++
		}
		first := sessions[missing[i]].Format(time.DateOnly)
		if j == i {
			parts = append(parts, first)
		} else {
			parts = append(parts, first+" to "+sessions[missing[j]].Format(time.DateOnly))
		}
		i = j + 1
	}
	return strings.Join(parts, ", ")
}

// listedOrder keeps the benchmark, which ingestSymbols puts first, and sorts
// the rest A to Z. The order a symbol list was written in is not stored, and
// ETF members are already read A to Z, so this is the same order every time.
func listedOrder(symbols []string) []string {
	if len(symbols) == 0 {
		return symbols
	}
	rest := slices.Clone(symbols[1:])
	slices.Sort(rest)
	return append([]string{symbols[0]}, rest...)
}

const missingSessionsNextStep = "Try another provider with --provider, or change the backtest dates, " +
	"then run `fbull backtest create` again."

// missingSessionsMessage is the failed DOWNLOADING step's message. One symbol
// fits one sentence. Two or more get one line each under the lead, in the
// order given, with every day shown. The CLI prints the message as is.
func missingSessionsMessage(name, provider string, sessions []time.Time, gaps []symbolGaps, removed bool) string {
	saved := "The backtest wasn't saved."
	next := missingSessionsNextStep
	if !removed {
		saved = fmt.Sprintf("The backtest couldn't be removed, so run `fbull backtest delete %s` "+
			"before you run `fbull backtest create` again.", name)
		next = ""
	}
	if len(gaps) == 1 {
		gap := gaps[0]
		days := sessionRanges(sessions, gap.sessions) + ", a trading day"
		if len(gap.sessions) > 1 {
			days = fmt.Sprintf("%d trading days: %s", len(gap.sessions), sessionRanges(sessions, gap.sessions))
		}
		return strings.TrimSpace(fmt.Sprintf("Couldn't load data for backtest '%s': %s is missing prices for %s on %s. %s %s",
			name, provider, gap.symbol, days, saved, next))
	}
	lines := []string{fmt.Sprintf("Couldn't load data for backtest '%s': %s is missing prices on trading days for %d symbols. %s",
		name, provider, len(gaps), saved)}
	for _, gap := range gaps {
		lines = append(lines, fmt.Sprintf("  %s: %s", gap.symbol, sessionRanges(sessions, gap.sessions)))
	}
	if next != "" {
		lines = append(lines, next)
	}
	return strings.Join(lines, "\n")
}

// checkMissingSessions fails create when a symbol has no bar on a session
// inside its own span, so a backtest never runs on a price nobody sent (see
// universe.md §7.5). It runs after the download, before Ingest writes the
// bundle, and deletes the backtest that the CREATING step stored.
func checkMissingSessions(ctx context.Context, msg stream.Message, db *pgxpool.Pool, backtests repository.Backtest,
	name, id, provider string, symbols []string, start, end time.Time,
) error {
	stored := make([]string, 0, len(symbols))
	for _, symbol := range symbols {
		stored = append(stored, finance.StoredSymbol(symbol))
	}
	barDays := repository.BarDays{Conn: db}
	bars, err := barDays.List(ctx, provider, stored, start, end)
	if err != nil {
		return discardFailedIngest(ctx, backtests, name, id, fmt.Errorf("error reading bar days: %v", err))
	}
	if len(bars) == 0 {
		// Nothing stored to check, so Zipline isn't asked. Ingest reports a
		// symbol with no bars.
		return nil
	}
	// Ingest's failure copy, since both calls reach the same Zipline pod.
	eng, err := msg.Call(ctx, dependency.GetEngineKey)
	if err != nil {
		return discardFailedIngest(ctx, backtests, name, id, fmt.Errorf("error getting zipline engine: %v", err))
	}
	ziplineEngine, isEngine := eng.(engine.Engine)
	if !isEngine {
		return errors.New("error casting zipline engine")
	}
	sessions, err := ziplineEngine.Sessions(ctx, start, end)
	if err != nil {
		return discardFailedIngest(ctx, backtests, name, id, err)
	}
	gaps := missingSessions(sessions, listedOrder(symbols), bars)
	if len(gaps) == 0 {
		return nil
	}
	err = deleteAfterFailure(ctx, backtests, name, id)
	removed := err == nil || errors.Is(err, pgx.ErrNoRows)
	if !removed {
		log.Warn().Err(err).Str("backtest", name).Msg("error deleting backtest after missing sessions")
	}
	return plainSentence(missingSessionsMessage(name, provider, sessions, gaps, removed))
}
