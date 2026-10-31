package command

import "time"

// SetSessionClock fixes the clock the session check caps the period with,
// and returns a func that puts the real one back.
func SetSessionClock(now func() time.Time) func() {
	previous := sessionClock
	sessionClock = now
	return func() { sessionClock = previous }
}
