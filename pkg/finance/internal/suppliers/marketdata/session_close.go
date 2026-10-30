package marketdata

import (
	"time"
	// The broker image may have no zoneinfo; embed America/New_York.
	_ "time/tzdata"
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

// lastCompletedSessionEnd clamps a fetch end so that today's unfinished
// XNYS session is never asked for. A session counts as completed at
// 16:30 America/New_York (sessionFinalAt), not at the 16:00 close. Before
// that, the fetch ends at midnight New York time, so the last bar is the
// previous session. Two cases need no calendar:
//   - A holiday or weekend has no session, so the provider sends no bar.
//   - On an early-close day (13:00), the session counts as completed only
//     at 16:30. That leaves a finished session out for three and a half
//     hours. It never lets an unfinished one in.
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
