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

// LimitSpan writes d as the adjective before "limit". A whole number of
// hours reads in hours (1-hour, 2-hour); anything else reads in whole
// minutes (90-minute), never in Go's mixed form such as 1h30m. A duration
// that isn't a whole number of minutes rounds up, with at least 1, so a
// 30 s or 1 s test limit reads 1-minute and 90.5 minutes reads 91-minute.
func LimitSpan(d time.Duration) string {
	if d > 0 && d%time.Hour == 0 {
		return Thousands(int(d/time.Hour)) + "-hour"
	}
	minutes := (d + time.Minute - 1) / time.Minute
	if minutes < 1 {
		minutes = 1
	}
	return Thousands(int(minutes)) + "-minute"
}
