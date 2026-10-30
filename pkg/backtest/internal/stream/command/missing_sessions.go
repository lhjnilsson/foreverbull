package command

import (
	"context"
	"errors"
	"fmt"
	"regexp"
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
	"github.com/lhjnilsson/foreverbull/pkg/finance/supplier"
	pb "github.com/lhjnilsson/foreverbull/pkg/pb"
	"github.com/lhjnilsson/foreverbull/pkg/pb/backtest"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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

// ziplineMissingSessions matches the reason of Zipline's MissingSessionsError,
// sent with DATA_LOSS: "missing prices on trading days: AAA 2024-02-14;
// BBB 2024-02-20,2024-03-05".
var ziplineMissingSessions = regexp.MustCompile(
	`^missing prices on trading days: ([A-Za-z0-9.^=_-]{1,32} \d{4}-\d{2}-\d{2}(?:,\d{4}-\d{2}-\d{2})*` +
		`(?:; [A-Za-z0-9.^=_-]{1,32} \d{4}-\d{2}-\d{2}(?:,\d{4}-\d{2}-\d{2})*)*)$`)

// dataLossReason is the reason of a DATA_LOSS ingest error, cut before its
// details. It is false for any other error.
func dataLossReason(cause error) (string, bool) {
	// errors.As, not status.FromError, which puts the wrapping text in the
	// message (readIngestFailure does the same).
	var grpcErr interface{ GRPCStatus() *status.Status }
	if !errors.As(cause, &grpcErr) {
		return "", false
	}
	st := grpcErr.GRPCStatus()
	if st.Code() != codes.DataLoss {
		return "", false
	}
	reason, _, _ := strings.Cut(st.Message(), ingestDetailsMarker)
	return strings.TrimSpace(reason), true
}

// ingestMissingDays reads the gaps from the reason of Zipline's
// MissingSessionsError, keyed by the ticker Zipline names.
func ingestMissingDays(reason string) (map[string][]time.Time, bool) {
	match := ziplineMissingSessions.FindStringSubmatch(reason)
	if match == nil {
		return nil, false
	}
	gaps := make(map[string][]time.Time)
	for _, entry := range strings.Split(match[1], "; ") {
		symbol, list, _ := strings.Cut(entry, " ")
		for _, field := range strings.Split(list, ",") {
			day, err := time.Parse(time.DateOnly, field)
			if err != nil {
				return nil, false
			}
			gaps[symbol] = append(gaps[symbol], day)
		}
	}
	return gaps, true
}

// ingestMissingSessions turns Zipline's refusal of a sid with missing
// sessions into the create's missing-session sentence, so a gap that
// appeared after the download check reads the same as one it caught. It is
// nil when err is another failure, or when the sentence would not be the
// whole list: Zipline named a ticker that isn't in this backtest, or a day
// that isn't a session. The caller then uses the ingest failure copy. Every
// DATA_LOSS outcome is logged with Zipline's cause, redacted.
func ingestMissingSessions(ctx context.Context, eng engine.Engine, db *pgxpool.Pool, backtests repository.Backtest,
	b *backtest.Backtest, id string, cause error,
) error {
	reason, isDataLoss := dataLossReason(cause)
	if !isDataLoss {
		return nil
	}
	outcome, refused := refuseMissingSessions(ctx, eng, db, backtests, b, id, reason)
	log.Warn().Str("backtest", b.Name).Str("outcome", outcome).Str("cause", redactURLPasswords(cause.Error())).
		Msg("zipline refused the ingest with data loss")
	return refused
}

// refuseMissingSessions builds the sentence for ingestMissingSessions, and
// says why it didn't when it is nil.
func refuseMissingSessions(ctx context.Context, eng engine.Engine, db *pgxpool.Pool, backtests repository.Backtest,
	b *backtest.Backtest, id, reason string,
) (string, error) {
	reported, ok := ingestMissingDays(reason)
	if !ok {
		return "not a missing-session refusal", nil
	}
	symbols, err := ingestSymbols(ctx, db, b, id)
	if err != nil {
		return "could not read the backtest's symbols", nil
	}
	start := pb.DateToTime(b.Periods[0].StartDate)
	end := pb.DateToTime(b.Periods[len(b.Periods)-1].EndDate)
	sessions, err := eng.Sessions(ctx, start, end)
	if err != nil {
		return "could not read the calendar", nil
	}
	known := make(map[string]bool, len(symbols))
	for _, symbol := range symbols {
		known[symbol] = true
	}
	isSession := make(map[time.Time]bool, len(sessions))
	for _, session := range sessions {
		isSession[session] = true
	}
	for symbol, days := range reported {
		if !known[symbol] {
			return "named a ticker that isn't in the backtest", nil
		}
		for _, day := range days {
			if !isSession[day] {
				return "named a day that isn't a session", nil
			}
		}
	}
	gaps := make([]symbolGaps, 0)
	for _, symbol := range listedOrder(symbols) {
		named := make(map[time.Time]bool)
		for _, day := range reported[symbol] {
			named[day] = true
		}
		var missing []int
		for i, session := range sessions {
			if named[session] {
				missing = append(missing, i)
			}
		}
		if len(missing) > 0 {
			gaps = append(gaps, symbolGaps{symbol: symbol, sessions: missing})
		}
	}
	if len(gaps) == 0 {
		return "named no days", nil
	}
	err = deleteAfterFailure(ctx, backtests, b.Name, id)
	removed := err == nil || errors.Is(err, pgx.ErrNoRows)
	if !removed {
		log.Warn().Err(err).Str("backtest", b.Name).Msg("error deleting backtest after missing sessions")
	}
	provider := strings.ToLower(strings.TrimSpace(b.GetProvider()))
	if provider == "" {
		provider = supplier.DefaultProvider
	}
	return "missing sessions", plainSentence(missingSessionsMessage(b.Name, provider, sessions, gaps, removed))
}
