package worker

import (
	"testing"
	"time"

	// The closes below are 16:00 America/New_York.
	_ "time/tzdata"

	"github.com/stretchr/testify/require"
)

func newYork() *time.Location {
	location, err := time.LoadLocation("America/New_York")
	if err != nil {
		panic(err)
	}
	return location
}

func sessionOn(year int, month time.Month, day int) CalendarSession {
	return CalendarSession{
		Date:  time.Date(year, month, day, 0, 0, 0, 0, time.UTC),
		Close: time.Date(year, month, day, 16, 0, 0, 0, newYork()),
	}
}

func TestLastFinishedSessionSkipsDaysThatAreNotSessions(t *testing.T) {
	t.Parallel()
	ny := newYork()

	t.Run("monday morning is friday", func(t *testing.T) {
		t.Parallel()
		got, err := LastFinishedSession([]CalendarSession{
			sessionOn(2026, time.November, 13),
			sessionOn(2026, time.November, 16),
		}, time.Date(2026, time.November, 16, 10, 0, 0, 0, ny))
		require.NoError(t, err)
		require.Equal(t, "2026-11-13", got.Format(time.DateOnly))
	})

	t.Run("morning after thanksgiving is the session before the holiday", func(t *testing.T) {
		t.Parallel()
		// 2026-11-26 is Thanksgiving and is not a row.
		got, err := LastFinishedSession([]CalendarSession{
			sessionOn(2026, time.November, 25),
			sessionOn(2026, time.November, 27),
		}, time.Date(2026, time.November, 27, 10, 0, 0, 0, ny))
		require.NoError(t, err)
		require.Equal(t, "2026-11-25", got.Format(time.DateOnly))
	})

	t.Run("saturday after 16:30 is friday", func(t *testing.T) {
		t.Parallel()
		got, err := LastFinishedSession([]CalendarSession{
			sessionOn(2026, time.November, 12),
			sessionOn(2026, time.November, 13),
		}, time.Date(2026, time.November, 14, 17, 0, 0, 0, ny))
		require.NoError(t, err)
		require.Equal(t, "2026-11-13", got.Format(time.DateOnly))
	})
}

func TestLastFinishedSessionUsesTheFinalityMargin(t *testing.T) {
	t.Parallel()
	friday := sessionOn(2026, time.November, 13)
	finished := friday.Close.Add(sessionFinality)
	got, err := LastFinishedSession([]CalendarSession{friday}, finished)
	require.NoError(t, err)
	require.Equal(t, "2026-11-13", got.Format(time.DateOnly))

	_, err = LastFinishedSession([]CalendarSession{friday}, finished.Add(-time.Nanosecond))
	require.ErrorIs(t, err, ErrNoCompletedSession)
}

func TestLastFinishedSessionHasNoDayToGuess(t *testing.T) {
	t.Parallel()
	_, err := LastFinishedSession(nil, time.Date(2026, time.November, 16, 10, 0, 0, 0, time.UTC))
	require.ErrorIs(t, err, ErrNoCompletedSession)
}
