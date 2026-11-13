package command

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/repository"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func copyDay(t *testing.T, value string) time.Time {
	t.Helper()
	day, err := time.Parse(time.DateOnly, value)
	require.NoError(t, err)
	return day
}

// The copy for a gap at the start of the period, word for word.
func TestMissingSessionsMessageStartGap(t *testing.T) {
	t.Parallel()
	gaps := []symbolGaps{{symbol: "NEWCO", noneBefore: copyDay(t, "2024-02-15")}}
	require.Equal(t, "Couldn't load data for backtest 'demo': NEWCO has no prices before 2024-02-15, and "+
		"yahoo didn't confirm it was listed then, so foreverbull treats it as missing data. "+
		"The backtest wasn't saved. "+
		"Choose a start on or after 2024-02-15, or remove NEWCO, then run `fbull backtest create` again.",
		missingSessionsMessage("demo", "yahoo", nil, gaps, true))
}

// The copy for a gap at the end of the period, word for word.
func TestMissingSessionsMessageEndGap(t *testing.T) {
	t.Parallel()
	gaps := []symbolGaps{{symbol: "OLDCO", noneAfter: copyDay(t, "2024-03-27")}}
	require.Equal(t, "Couldn't load data for backtest 'demo': OLDCO has no prices after 2024-03-27, and "+
		"yahoo didn't confirm it was delisted then, so foreverbull treats it as missing data. "+
		"The backtest wasn't saved. "+
		"Choose an end on or before 2024-03-27, or remove OLDCO, then run `fbull backtest create` again.",
		missingSessionsMessage("demo", "yahoo", nil, gaps, true))
}

// When the backtest couldn't be removed, it was partly saved, and the step
// is its own sentence to delete it first, then the error's own step as it
// reads when the backtest was removed.
func TestMissingSessionsMessageNotRemoved(t *testing.T) {
	t.Parallel()
	start := []symbolGaps{{symbol: "NEWCO", noneBefore: copyDay(t, "2024-02-15")}}
	require.Equal(t, "Couldn't load data for backtest 'oef-2024q1': NEWCO has no prices before 2024-02-15, and "+
		"yahoo didn't confirm it was listed then, so foreverbull treats it as missing data. "+
		"The backtest was partly saved and couldn't be removed. Run `fbull backtest delete oef-2024q1` first. "+
		"Choose a start on or after 2024-02-15, or remove NEWCO, then run `fbull backtest create` again.",
		missingSessionsMessage("oef-2024q1", "yahoo", nil, start, false))

	end := []symbolGaps{{symbol: "OLDCO", noneAfter: copyDay(t, "2024-03-27")}}
	require.Equal(t, "Couldn't load data for backtest 'oef-2024q1': OLDCO has no prices after 2024-03-27, and "+
		"yahoo didn't confirm it was delisted then, so foreverbull treats it as missing data. "+
		"The backtest was partly saved and couldn't be removed. Run `fbull backtest delete oef-2024q1` first. "+
		"Choose an end on or before 2024-03-27, or remove OLDCO, then run `fbull backtest create` again.",
		missingSessionsMessage("oef-2024q1", "yahoo", nil, end, false))

	days := []time.Time{copyDay(t, "2024-02-14"), copyDay(t, "2024-02-20")}

	several := []symbolGaps{{symbol: "AAA", sessions: []int{1}}, {symbol: "NEWCO", noneBefore: days[1]}}
	require.Equal(t, "Couldn't load data for backtest 'oef-2024q1': yahoo is missing prices on trading days for "+
		"2 symbols. The backtest was partly saved and couldn't be removed.\n"+
		"  AAA: 2024-02-20\n"+
		"  NEWCO: no prices before 2024-02-20.\n"+
		"yahoo didn't confirm a listing, so foreverbull treats it as missing data. "+
		"Run `fbull backtest delete oef-2024q1` first. Try another provider with --provider, or change the "+
		"backtest dates, then run `fbull backtest create` again.",
		missingSessionsMessage("oef-2024q1", "yahoo", days, several, false))

	noBars := []symbolGaps{{symbol: "XYZ", noBars: true, from: days[0], to: days[1]}}
	require.Equal(t, "Couldn't load data for backtest 'oef-2024q1': yahoo has no prices for XYZ between 2024-02-14 "+
		"and 2024-02-20. The backtest was partly saved and couldn't be removed. Run `fbull backtest delete oef-2024q1` "+
		"first. Check the symbol and dates, or try another provider with --provider, then run "+
		"`fbull backtest create` again.",
		missingSessionsMessage("oef-2024q1", "yahoo", days, noBars, false))
}

// Request symbols of one instrument are one unit, named by its latest
// ticker; any other symbol is read alone, named as requested.
func TestCheckUnits(t *testing.T) {
	t.Parallel()
	fb := repository.Ticker{Symbol: "FB", From: copyDay(t, "2012-05-18"), To: copyDay(t, "2022-06-09")}
	meta := repository.Ticker{Symbol: "META", From: copyDay(t, "2012-05-18")}
	units, alias := checkUnits([]string{"OEF", "BRK.B", "FB", "META"},
		map[string][]repository.Ticker{"FB": {fb, meta}, "META": {fb, meta}})
	require.Equal(t, []checkUnit{
		{name: "OEF", tickers: []repository.Ticker{{Symbol: "OEF"}}},
		{name: "BRK.B", tickers: []repository.Ticker{{Symbol: "BRKB"}}},
		{name: "META (earlier FB)", tickers: []repository.Ticker{fb, meta}},
	}, units)
	require.Equal(t, "META (earlier FB)", alias["META"])
}

// Twenty symbols are named. The twenty-first is counted, and the benchmark
// stays first even when its ticker sorts last. The log keeps that symbol.
func TestMissingSessionsMessageCapsSymbols(t *testing.T) {
	days := []time.Time{copyDay(t, "2024-02-14")}
	gaps := make([]symbolGaps, 21)
	for i := range gaps {
		name := fmt.Sprintf("S%02d", i+1)
		if i == 0 {
			name = "ZZZ"
		}
		gaps[i] = symbolGaps{symbol: name, sessions: []int{0}}
	}
	var buf bytes.Buffer
	previous := log.Logger
	log.Logger = zerolog.New(&buf)
	t.Cleanup(func() { log.Logger = previous })

	message := presentMissingSessions("demo", "yahoo", days, gaps, true, nil, 0)
	require.Contains(t, message, "yahoo is missing prices on trading days for 21 symbols.")
	require.Contains(t, message, "  ZZZ: 2024-02-14\n")
	require.Contains(t, message, "  S20: 2024-02-14\n")
	require.NotContains(t, message, "S21")
	require.Contains(t, message, "  … and 1 more symbol\n")
	require.NotContains(t, message, "more symbol is missing")
	require.Contains(t, buf.String(), "S21")
	require.Contains(t, buf.String(), "missing sessions omitted from the error")
}

// Twenty-five symbols name the exact remainder, and a symbol's ranges stop
// at ten. Isolated sessions are one range each; a run is one range and is
// not capped.
func TestMissingSessionsMessageCapsRanges(t *testing.T) {
	days := make([]time.Time, 40)
	for i := range days {
		days[i] = copyDay(t, "2024-01-02").AddDate(0, 0, i)
	}
	isolated := make([]int, 14)
	for i := range isolated {
		isolated[i] = i * 2
	}
	message := missingSessionsMessage("demo", "yahoo", days, []symbolGaps{
		{symbol: "ZZZ", sessions: isolated},
	}, true)
	shown := make([]string, 10)
	for i := range shown {
		shown[i] = days[i*2].Format(time.DateOnly)
	}
	require.Equal(t, "Couldn't load data for backtest 'demo': yahoo is missing prices for ZZZ on more than "+
		"10 trading days: "+strings.Join(shown, ", ")+" … and 4 more ranges. The backtest wasn't saved. "+
		"Try another provider with --provider, or change the backtest dates, then run `fbull backtest create` again.",
		message)

	run := make([]int, 11)
	for i := range run {
		run[i] = i
	}
	whole := missingSessionsMessage("demo", "yahoo", days, []symbolGaps{
		{symbol: "ZZZ", sessions: run},
	}, true)
	require.Contains(t, whole, "on 11 trading days: 2024-01-02 to 2024-01-12.")
	require.NotContains(t, whole, "more range")
}

// Exactly 20 symbols and exactly 10 ranges say nothing was left out.
// One more of either names that one.
func TestGapCapBoundaries(t *testing.T) {
	t.Parallel()
	days := make([]time.Time, 40)
	for i := range days {
		days[i] = copyDay(t, "2024-01-02").AddDate(0, 0, i)
	}
	one := []symbolGaps{}
	for i := range 20 {
		name := fmt.Sprintf("S%02d", i+1)
		if i == 0 {
			name = "ZZZ"
		}
		one = append(one, symbolGaps{symbol: name, sessions: []int{0}})
	}
	exact := missingSessionsMessage("demo", "yahoo", days, one, true)
	require.NotContains(t, exact, "more symbol")
	require.Contains(t, exact, "for 20 symbols.")
	one = append(one, symbolGaps{symbol: "S21", sessions: []int{0}})
	over := missingSessionsMessage("demo", "yahoo", days, one, true)
	require.Contains(t, over, "  … and 1 more symbol\n")

	ten := make([]int, 10)
	for i := range ten {
		ten[i] = i * 2
	}
	ranges := missingSessionsMessage("demo", "yahoo", days, []symbolGaps{{symbol: "ZZZ", sessions: ten}}, true)
	require.Contains(t, ranges, "on 10 trading days: ")
	require.NotContains(t, ranges, "more than")
	require.NotContains(t, ranges, "more range")
	ten = append(ten, 20)
	overRanges := missingSessionsMessage("demo", "yahoo", days, []symbolGaps{{symbol: "ZZZ", sessions: ten}}, true)
	require.Contains(t, overRanges, "on more than 10 trading days: ")
	require.Contains(t, overRanges, " … and 1 more range.")
}

// The detail lines stop at 20 symbols. The step that says what to remove
// names every symbol, and only the names.
func TestEdgeStepNamesEverySymbol(t *testing.T) {
	t.Parallel()
	gaps := make([]symbolGaps, 25)
	for i := range gaps {
		gaps[i] = symbolGaps{symbol: fmt.Sprintf("S%02d", i+1), noneBefore: copyDay(t, "2024-12-01")}
		if i%2 == 1 {
			gaps[i].noneBefore = time.Time{}
			gaps[i].noneAfter = copyDay(t, "2024-02-01")
		}
	}
	step := edgeStep(gaps, copyDay(t, "2024-01-02"), copyDay(t, "2024-12-31"))
	require.Contains(t, step, "Remove S01, S02, S03")
	require.Contains(t, step, "and S25, then run `fbull backtest create` again.")
	require.NotContains(t, step, "more")
	require.NotContains(t, step, "2024-")
}

// Zipline already capped the reason. The sentence keeps those counts and
// does not pretend the short list is the whole one.
func TestPresentMissingSessionsKeepsParsedLeftovers(t *testing.T) {
	t.Parallel()
	days := []time.Time{copyDay(t, "2024-02-14"), copyDay(t, "2024-02-20"), copyDay(t, "2024-02-21")}
	message := presentMissingSessions("oef-2024q1", "yahoo", days, []symbolGaps{
		{symbol: "AAA", sessions: []int{0}},
		{symbol: "BBB", sessions: []int{1, 2}},
	}, true, map[string]int{"BBB": 4}, 37)
	require.Contains(t, message, "for 39 symbols.")
	require.Contains(t, message, "  AAA: 2024-02-14\n")
	require.Contains(t, message, "  BBB: 2024-02-20 to 2024-02-21 … and 4 more ranges\n")
	require.Contains(t, message, "  … and 37 more symbols\n")
	require.NotContains(t, message, "more symbols are missing")
}

// A capped DATA_LOSS reason still parses, and the leftovers keep their counts.
func TestIngestGapReasonReadsACappedList(t *testing.T) {
	t.Parallel()
	spans, ranges, symbols, ok := ingestGapReason(
		"missing prices on trading days: AAA 2024-02-14; BBB 2024-02-20..2024-02-21 and 4 more ranges; and 37 more symbols")
	require.True(t, ok)
	require.Equal(t, []daySpan{{from: copyDay(t, "2024-02-14"), to: copyDay(t, "2024-02-14")}}, spans["AAA"])
	require.Equal(t, []daySpan{{from: copyDay(t, "2024-02-20"), to: copyDay(t, "2024-02-21")}}, spans["BBB"])
	require.Equal(t, 4, ranges["BBB"])
	require.Equal(t, 0, ranges["AAA"])
	require.Equal(t, 37, symbols)
	// The old day list still parses, so a reason from before ranges does too.
	listed, _, _, ok := ingestGapReason(
		"missing prices on trading days: AAA 2024-02-14; BBB 2024-02-20,2024-02-21")
	require.True(t, ok)
	require.Equal(t, []daySpan{
		{from: copyDay(t, "2024-02-20"), to: copyDay(t, "2024-02-20")},
		{from: copyDay(t, "2024-02-21"), to: copyDay(t, "2024-02-21")},
	}, listed["BBB"])

	raw, err := os.ReadFile("testdata/capped_gap_reason.txt")
	require.NoError(t, err)
	records := strings.Split(strings.TrimSpace(string(raw)), "\n\n")
	require.Len(t, records, 2)
	reason := records[0]
	require.LessOrEqual(t, len(reason+"\nDetails: MissingSessionsError"), gapReasonByteBound)
	require.Less(t, len(reason), 128)
	spanReason, _, _, ok := ingestGapReason(reason)
	require.True(t, ok)
	from, to := spanReason["AAA"][0].from, spanReason["AAA"][0].to
	var calendar []time.Time
	for day := from; !day.After(to); day = day.AddDate(0, 0, 1) {
		if day.Weekday() == time.Saturday || day.Weekday() == time.Sunday {
			continue
		}
		calendar = append(calendar, day)
	}
	reported, ok := expandDaySpans(calendar, spanReason)
	require.True(t, ok)
	_, gaps := missingSessions(calendar, []string{"AAA"}, nil, reported, false)
	sentence := presentMissingSessions("oef-2024q1", "yahoo", calendar, gaps, true, nil, 0)
	require.Contains(t, sentence, "yahoo is missing prices for AAA on ")
	require.Contains(t, sentence, "2020-01-02 to 2022-12-30. ")
	require.Contains(t, sentence, "The backtest wasn't saved.")
	require.NotContains(t, sentence, "more range")

	many := records[1]
	require.LessOrEqual(t, len(many+"\nDetails: MissingSessionsError"), gapReasonByteBound)
	manySpans, manyRanges, manySymbols, ok := ingestGapReason(many)
	require.True(t, ok)
	require.Equal(t, 1, manyRanges["ZZZ"])
	require.Equal(t, 1, manySymbols)
	require.Len(t, manySpans, 20)
	require.NotContains(t, many, "S19")
	var first, last time.Time
	for _, list := range manySpans {
		for _, span := range list {
			if first.IsZero() || span.from.Before(first) {
				first = span.from
			}
			if span.to.After(last) {
				last = span.to
			}
		}
	}
	var weekdays []time.Time
	for day := first; !day.After(last); day = day.AddDate(0, 0, 1) {
		if day.Weekday() == time.Saturday || day.Weekday() == time.Sunday {
			continue
		}
		weekdays = append(weekdays, day)
	}
	reportedMany, ok := expandDaySpans(weekdays, manySpans)
	require.True(t, ok)
	names := []string{"ZZZ"}
	for symbol := range reportedMany {
		if symbol != "ZZZ" {
			names = append(names, symbol)
		}
	}
	_, manyGaps := missingSessions(weekdays, names, nil, reportedMany, false)
	manySentence := presentMissingSessions("oef-2024q1", "yahoo", weekdays, manyGaps, true, manyRanges, manySymbols)
	require.Contains(t, manySentence, "for 21 symbols.")
	require.Contains(t, manySentence, "  ZZZ: ")
	require.Contains(t, manySentence, " … and 1 more range\n")
	require.Contains(t, manySentence, "  … and 1 more symbol\n")
	require.NotContains(t, manySentence, "S19")
	require.Contains(t, manySentence, "The backtest wasn't saved.")

	_, _, _, ok = ingestGapReason("missing prices on trading days: AAA 2024-02-14 and then some")
	require.False(t, ok)
	_, _, _, ok = ingestGapReason("something else")
	require.False(t, ok)
}

// A DATA_LOSS status longer than the bound is not the missing-session sentence.
func TestDataLossReasonRejectsAStatusOverTheBound(t *testing.T) {
	t.Parallel()
	reason := "missing prices on trading days: AAA 2024-02-14\nDetails: MissingSessionsError"
	_, ok := dataLossReason(status.Error(codes.DataLoss, reason))
	require.True(t, ok)
	_, ok = dataLossReason(status.Error(codes.DataLoss, reason+strings.Repeat("x", gapReasonByteBound)))
	require.False(t, ok)
}

// The session check sees an edge gap only for a session of the period:
// a symbol with bars over the whole period has none, and one whose bars
// start late or end early has it at its first or last bar.
func TestMissingSessionsFindsEdgeGaps(t *testing.T) {
	t.Parallel()
	sessions := []time.Time{copyDay(t, "2024-02-13"), copyDay(t, "2024-02-14"), copyDay(t, "2024-02-15")}
	_, gaps := missingSessions(sessions, []string{"FULL", "LATE", "EARLY"}, map[string]*repository.SymbolBars{
		"FULL":  {First: sessions[0], Last: sessions[2], Days: sessions},
		"LATE":  {First: sessions[1], Last: sessions[2], Days: sessions[1:]},
		"EARLY": {First: sessions[0], Last: sessions[1], Days: sessions[:2]},
	}, nil, true)
	require.Len(t, gaps, 2)
	require.Equal(t, "LATE", gaps[0].symbol)
	require.Equal(t, sessions[1], gaps[0].noneBefore)
	require.Empty(t, gaps[0].sessions)
	require.Equal(t, "EARLY", gaps[1].symbol)
	require.Equal(t, sessions[1], gaps[1].noneAfter)
	require.Empty(t, gaps[1].sessions)
}

// An empty period that couldn't be removed says so the usual way: partly
// saved, delete first, then the step as it reads.
func TestEmptyPeriodMessages(t *testing.T) {
	t.Parallel()
	today := copyDay(t, "2026-10-30")
	require.Equal(t, "Couldn't load data for backtest 'oef-2024q1': no trading session between 2026-10-30 and "+
		"2026-10-30 has closed yet (sessions count as closed at 16:30 New York time). The backtest wasn't saved. "+
		"Choose a start on or before 2026-10-29, then run `fbull backtest create` again.",
		notClosedMessage("oef-2024q1", today, today, copyDay(t, "2026-10-29"), true))
	require.Equal(t, "Couldn't load data for backtest 'oef-2024q1': no trading session between 2026-10-30 and "+
		"2026-10-30 has closed yet (sessions count as closed at 16:30 New York time). The backtest was partly "+
		"saved and couldn't be removed. Run `fbull backtest delete oef-2024q1` first. Choose a start on or before "+
		"2026-10-29, then run `fbull backtest create` again.",
		notClosedMessage("oef-2024q1", today, today, copyDay(t, "2026-10-29"), false))
	require.Equal(t, "Couldn't load data for backtest 'oef-2024q1': there's no trading session between 2024-03-30 "+
		"and 2024-03-31. The backtest wasn't saved. Choose dates that include a trading day, then run "+
		"`fbull backtest create` again.",
		noSessionMessage("oef-2024q1", copyDay(t, "2024-03-30"), copyDay(t, "2024-03-31"), true))
	require.Equal(t, "Couldn't load data for backtest 'oef-2024q1': there's no trading session between 2024-03-30 "+
		"and 2024-03-31. The backtest was partly saved and couldn't be removed. Run `fbull backtest delete "+
		"oef-2024q1` first. Choose dates that include a trading day, then run `fbull backtest create` again.",
		noSessionMessage("oef-2024q1", copyDay(t, "2024-03-30"), copyDay(t, "2024-03-31"), false))
}

// A backtest that ends after the market calendar's last trading day names
// both dates, whether or not it could be removed.
func TestPastCalendarMessage(t *testing.T) {
	t.Parallel()
	end, last := copyDay(t, "2027-12-17"), copyDay(t, "2027-11-02")
	require.Equal(t, "Couldn't load data for backtest 'oef-2024q1': it ends on 2027-12-17, after the last trading "+
		"day in foreverbull's market calendar (2027-11-02). The backtest wasn't saved. Choose an end on or before "+
		"2027-11-02, then run `fbull backtest create` again.",
		pastCalendarMessage("oef-2024q1", end, last, true))
	require.Equal(t, "Couldn't load data for backtest 'oef-2024q1': it ends on 2027-12-17, after the last trading "+
		"day in foreverbull's market calendar (2027-11-02). The backtest was partly saved and couldn't be removed. "+
		"Run `fbull backtest delete oef-2024q1` first. Choose an end on or before 2027-11-02, then run "+
		"`fbull backtest create` again.",
		pastCalendarMessage("oef-2024q1", end, last, false))
}
