package engine

import (
	"fmt"
	"time"
)

// CalendarStartError is a Sessions read before the first session of the
// engine's calendar. Start is the requested start and FirstSession is that
// first session, both at midnight UTC.
type CalendarStartError struct {
	Calendar     string
	Start        time.Time
	FirstSession time.Time
}

func (e *CalendarStartError) Error() string {
	return fmt.Sprintf("%s is before the %s calendar's first session %s",
		e.Start.Format(time.DateOnly), e.Calendar, e.FirstSession.Format(time.DateOnly))
}
