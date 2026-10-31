// Package numfmt formats numbers for user-facing sentences.
package numfmt

import (
	"strconv"
	"strings"
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
