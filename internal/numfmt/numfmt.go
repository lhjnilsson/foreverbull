// Package numfmt formats numbers for user-facing sentences.
package numfmt

import (
	"strconv"
	"strings"
	"time"
)

// Thousands writes n with a comma between each group of three digits, for
// example 10,000 or 6,214.
func Thousands(n int) string {
	if n < 0 {
		return "-" + Thousands(-n)
	}
	digits := strconv.Itoa(n)
	var b strings.Builder
	for i, d := range digits {
		if i > 0 && (len(digits)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(d)
	}
	return b.String()
}

// LimitSpan writes d as the adjective before "limit": 2-hour, 90-minute or
// 1-second. A duration that isn't a whole number of seconds keeps Go's
// form, such as 1.5s.
func LimitSpan(d time.Duration) string {
	switch {
	case d > 0 && d%time.Hour == 0:
		return Thousands(int(d/time.Hour)) + "-hour"
	case d > 0 && d%time.Minute == 0:
		return Thousands(int(d/time.Minute)) + "-minute"
	case d > 0 && d%time.Second == 0:
		return Thousands(int(d/time.Second)) + "-second"
	default:
		return d.String()
	}
}
