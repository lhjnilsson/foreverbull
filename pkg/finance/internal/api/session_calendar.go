package api

import (
	"context"
	"errors"
	"time"

	"github.com/lhjnilsson/foreverbull/internal/postgres"
	"github.com/lhjnilsson/foreverbull/pkg/service/worker"
	"github.com/rs/zerolog/log"
)

const (
	xnysCalendarCode = "XNYS"
	// finishedSessionRows is the latest close at or before asOf, and the
	// one before it. The later close can still be inside the 30-minute
	// margin. The earlier close is a session day before that, so it has
	// finished. A third row is not read.
	finishedSessionRows = 2
)

// xnysCalendar reads trading_calendar_date for XNYS, the same table the
// strategy runner's CalendarClock reads.
type xnysCalendar struct {
	conn postgres.Query
}

// NewXNYSCalendar is the session calendar a trading step uses.
func NewXNYSCalendar(conn postgres.Query) worker.SessionCalendar {
	return xnysCalendar{conn: conn}
}

// LastSession is the latest XNYS session whose close plus the 30-minute
// finality margin is at or before asOf. No such row is
// worker.ErrNoCompletedSession. A nil connection is
// worker.ErrCalendarNotConfigured. A query failure is
// worker.ErrCalendarRead, and its text is not the database error. A
// canceled or timed-out read is the context error, not that sentence.
func (c xnysCalendar) LastSession(ctx context.Context, asOf time.Time) (time.Time, error) {
	if c.conn == nil {
		return time.Time{}, worker.CalendarNotConfigured()
	}
	rows, err := c.conn.Query(ctx, `
		SELECT date, close FROM trading_calendar_date
		WHERE code = $1 AND close <= $2
		ORDER BY date DESC
		LIMIT $3`,
		xnysCalendarCode, asOf.UTC(), finishedSessionRows,
	)
	if err != nil {
		return time.Time{}, calendarReadResult(ctx, err)
	}
	defer rows.Close()

	sessions := make([]worker.CalendarSession, 0, finishedSessionRows)
	for rows.Next() {
		if len(sessions) == finishedSessionRows {
			break
		}
		var date, closeAt time.Time
		if err := rows.Scan(&date, &closeAt); err != nil {
			return time.Time{}, calendarReadResult(ctx, err)
		}
		sessions = append(sessions, worker.CalendarSession{Date: date, Close: closeAt})
	}
	if err := rows.Err(); err != nil {
		return time.Time{}, calendarReadResult(ctx, err)
	}
	return worker.LastFinishedSession(sessions, asOf)
}

// calendarReadResult returns a canceled or timed-out context as itself.
// Any other failure is the calendar sentence, with the database error logged
// and kept on the unwrap chain.
func calendarReadResult(ctx context.Context, err error) error {
	if stopped := contextStop(ctx, err); stopped != nil {
		return stopped
	}
	log.Error().Err(err).Str("calendar", xnysCalendarCode).Msg("couldn't read the trading calendar")
	return worker.CalendarReadFailed(err)
}

// contextStop is the context's own error when the read stopped because
// the context was canceled or its deadline passed.
func contextStop(ctx context.Context, err error) error {
	if ctx != nil {
		switch ctx.Err() {
		case context.Canceled, context.DeadlineExceeded:
			return ctx.Err()
		}
	}
	// The driver often wraps the sentinel in SQL text. Return the sentinel
	// itself so that text is not the error a caller shows.
	if errors.Is(err, context.Canceled) {
		return context.Canceled
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return context.DeadlineExceeded
	}
	return nil
}
