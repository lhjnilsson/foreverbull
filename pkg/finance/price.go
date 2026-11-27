package finance

import (
	"time"

	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/pkg/finance/internal/suppliers/marketdata"
)

// ValidatePriceSource checks a backtest price source before download.
// yahoo and stooq only check the name. alpaca resolves a profile from the
// profiles file and fails when that profile cannot be chosen.
// It does not construct a Yahoo client and does not download bars.
func ValidatePriceSource(provider, profile string) error {
	return marketdata.ValidatePriceSource(provider, profile, environment.GetTradingProfilesFile())
}

// StoredSymbol is the one name a share class is stored under. BRK.B, BRK-B,
// BRK/B and BRK B all store as BRKB. A joined iShares ticker is that name
// already. Every other symbol, including an exchange suffix such as VOD.L,
// is returned exactly as typed.
func StoredSymbol(symbol string) string {
	return marketdata.StoredSymbol(symbol)
}

// LearnShareClasses records separated share-class spellings before they are
// folded to the stored name. BRK.B teaches BRKB, so a later ask for the
// stored name uses BRK-B.
func LearnShareClasses(symbols ...string) {
	marketdata.LearnShareClasses(symbols...)
}

// LearnJoinedShareClass records asked when hits contain its separated
// spelling. The hit comes from the ISIN's listing or a provider search.
func LearnJoinedShareClass(asked string, hits ...string) {
	marketdata.LearnJoinedShareClass(asked, hits...)
}

// JoinedShareClass reports a ticker iShares may have written without a
// separator, such as BRKB.
func JoinedShareClass(symbol string) bool {
	return marketdata.JoinedShareClass(symbol)
}

// YahooSymbol is the ticker Yahoo is asked for after LearnShareClasses.
func YahooSymbol(symbol string) string {
	return marketdata.YahooSymbol(symbol)
}

// StooqExchangeLead is Stooq's refusal of symbol without the provider step.
// ok is false when symbol has no exchange suffix. Create uses it as the
// lead; the full GetOHLC sentence is this plus " Try another provider with
// --provider."
func StooqExchangeLead(symbol string) (string, bool) {
	return marketdata.StooqExchangeLead(symbol)
}

// IsStooqExchangeRefusal reports whether text is the full GetOHLC sentence
// for symbol, including that step.
func IsStooqExchangeRefusal(text, symbol string) bool {
	return marketdata.IsStooqExchangeRefusal(text, symbol)
}

// LastCompletedSessionDate is the date of the last completed XNYS session as
// of now, at midnight UTC: today from 16:30 America/New_York, yesterday
// before. Every provider's fetch stops there, so a check of stored bars
// stops there too.
func LastCompletedSessionDate(now time.Time) time.Time {
	return marketdata.LastCompletedSessionDate(now)
}

// NoClosedSessionReason says that no session from start to end has closed
// yet, naming the New York time a session counts as closed at, the same
// time every fetch stops at: "no trading session between 2026-10-31 and
// 2026-10-31 has closed yet (sessions count as closed at 16:30 New York
// time)". Use it wherever that is said, rather than writing it again.
func NoClosedSessionReason(start, end time.Time) string {
	return marketdata.NoClosedSessionReason(start, end)
}

// UnclosedSessionsNotice says that the sessions from first to last were
// left out of a backtest because they haven't closed yet, with the same
// closing time as NoClosedSessionReason: "2026-10-30 isn't included because
// its trading session hasn't closed yet (sessions count as closed at 16:30
// New York time)."
func UnclosedSessionsNotice(first, last time.Time) string {
	return marketdata.UnclosedSessionsNotice(first, last)
}
