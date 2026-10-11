package runner

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/lhjnilsson/foreverbull/internal/postgres"
	commonpb "github.com/lhjnilsson/foreverbull/pkg/pb"
)

// ErrNoSession is returned when the calendar has no further session close.
var ErrNoSession = errors.New("no calendar session")

// SessionClose is one trading day's close on a calendar.
type SessionClose struct {
	Date  *commonpb.Date
	Close time.Time
}

// Clock drives deployment schedules. Tests inject a fake; production uses
// CalendarClock (trading_calendar_date + wall clock).
type Clock interface {
	Now() time.Time
	NextClose(ctx context.Context, calendarCode string, after *commonpb.Date) (*SessionClose, error)
	WaitUntil(ctx context.Context, t time.Time) error
}

// CalendarClock reads session closes from trading_calendar_date and waits
// with time.Now.
type CalendarClock struct {
	conn postgres.Query
	now  func() time.Time
}

func NewCalendarClock(conn postgres.Query) *CalendarClock {
	return &CalendarClock{conn: conn, now: time.Now}
}

var _ Clock = (*CalendarClock)(nil)

func (c *CalendarClock) Now() time.Time {
	if c.now != nil {
		return c.now()
	}
	return time.Now()
}

func (c *CalendarClock) NextClose(ctx context.Context, calendarCode string, after *commonpb.Date) (*SessionClose, error) {
	var last any
	if after != nil {
		last = commonpb.DateToDateString(after)
	}

	var date time.Time
	var closeTime time.Time
	err := c.conn.QueryRow(ctx, `
		SELECT date, close FROM trading_calendar_date
		WHERE code = $1
		AND CASE
			WHEN $2::date IS NULL THEN close >= $3
			ELSE date > $2::date
		END
		ORDER BY date ASC
		LIMIT 1`,
		calendarCode, last, c.Now(),
	).Scan(&date, &closeTime)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNoSession
	}
	if err != nil {
		return nil, err
	}

	return &SessionClose{
		Date:  commonpb.GoTimeToDate(date),
		Close: closeTime,
	}, nil
}

func (c *CalendarClock) WaitUntil(ctx context.Context, t time.Time) error {
	d := t.Sub(c.Now())
	if d <= 0 {
		return nil
	}

	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func dateEqual(a, b *commonpb.Date) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return a.Year == b.Year && a.Month == b.Month && a.Day == b.Day
}

func dateAfter(a, b *commonpb.Date) bool {
	if a == nil || b == nil {
		return a != nil
	}
	if a.Year != b.Year {
		return a.Year > b.Year
	}
	if a.Month != b.Month {
		return a.Month > b.Month
	}
	return a.Day > b.Day
}
