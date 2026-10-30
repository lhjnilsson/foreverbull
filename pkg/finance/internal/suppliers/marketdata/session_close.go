package marketdata

import (
	"time"
	// The broker image may have no zoneinfo; embed America/New_York.
	_ "time/tzdata"

	"github.com/rs/zerolog/log"
)

// xnysZone is the exchange's time zone. LoadLocation cannot fail with the
// embedded zone database.
var xnysZone = mustLoadLocation("America/New_York")

// sessionFinalAt is when, in New York wall time, a session counts as
// completed: 30 minutes after the regular 16:00 close, so the closing
// auction has printed and providers have revised the day's bar into its
// final daily bar.
const sessionFinalAt = 16*time.Hour + 30*time.Minute

func mustLoadLocation(name string) *time.Location {
	location, err := time.LoadLocation(name)
	if err != nil {
		panic(err)
	}
	return location
}

// clockNow is now, or the time a test fixed.
func clockNow(now func() time.Time) time.Time {
	if now != nil {
		return now()
	}
	return time.Now()
}

// lastCompletedSessionEnd clamps a fetch end so that today's unfinished
// XNYS session is never asked for. A session counts as completed at
// 16:30 America/New_York (sessionFinalAt), not at the 16:00 close. Before
// that, the fetch ends at midnight New York time, so the last bar is the
// previous session. Two cases need no calendar:
//   - A holiday or weekend has no session, so the provider sends no bar.
//   - On an early-close day (13:00), the session counts as completed only
//     at 16:30. That leaves a finished session out for three and a half
//     hours. It never lets an unfinished one in.
//
// Every provider clamps with it: Yahoo, Alpaca and Stooq. A provider whose
// end is inclusive, or counted in days, stops just before the limit when it
// applies, so a bar stamped at midnight New York today is left out too.
func lastCompletedSessionEnd(end, now time.Time) time.Time {
	local := now.In(xnysZone)
	year, month, day := local.Date()
	finalAt := time.Date(year, month, day, int(sessionFinalAt/time.Hour),
		int(sessionFinalAt%time.Hour/time.Minute), 0, 0, xnysZone)
	limit := now
	if local.Before(finalAt) {
		limit = time.Date(year, month, day, 0, 0, 0, 0, xnysZone)
	}
	if end.After(limit) {
		return limit
	}
	return end
}

// completedFetchEnd is the clamped end of a fetch from start. It is false
// when nothing completed is left to fetch, because the clamped end is not
// after start, such as a fetch for today alone before the close. The
// provider is then not asked, and the fetch has no bars rather than an
// error.
func completedFetchEnd(start, end, now time.Time) (time.Time, bool) {
	until := lastCompletedSessionEnd(end, now)
	return until, until.After(start)
}

// lastCompletedSession is a time on the New York date of the last
// completed XNYS session as of now: today from 16:30 New York
// (sessionFinalAt), yesterday before. A weekend or holiday date has no
// session, so no bar is on it.
func lastCompletedSession(now time.Time) time.Time {
	return lastCompletedSessionEnd(now, now).Add(-time.Nanosecond).In(xnysZone)
}

// afterLastSession is true for a bar whose session date comes after last,
// the date of the last session the fetch may return, and logs the drop at
// debug level. Each date is read in its own location: Stooq's rows are
// dates at midnight UTC, Yahoo's and Alpaca's bars are read in New York.
// Asking a provider for completed sessions only is not enough: Stooq can
// send rows past d2, Yahoo's chart API sometimes sends today's live bar
// when period2 is earlier, and Alpaca could too. Every provider drops such
// a bar with it rather than store an unfinished session as final.
func afterLastSession(provider, symbol string, session, last time.Time) bool {
	sessionYear, sessionMonth, sessionDay := session.Date()
	lastYear, lastMonth, lastDay := last.Date()
	sessionDate := time.Date(sessionYear, sessionMonth, sessionDay, 0, 0, 0, 0, time.UTC)
	lastDate := time.Date(lastYear, lastMonth, lastDay, 0, 0, 0, 0, time.UTC)
	if !sessionDate.After(lastDate) {
		return false
	}
	log.Debug().
		Str("provider", provider).
		Str("symbol", symbol).
		Str("session", sessionDate.Format(time.DateOnly)).
		Str("last_session", lastDate.Format(time.DateOnly)).
		Msg("dropped a bar after the last completed session")
	return true
}
