package numfmt_test

import (
	"testing"
	"time"

	"github.com/lhjnilsson/foreverbull/internal/numfmt"
	"github.com/stretchr/testify/require"
)

func TestThousands(t *testing.T) {
	t.Parallel()
	for n, want := range map[int]string{
		0: "0", 999: "999", 1000: "1,000", 6214: "6,214", 10_000: "10,000",
		10_001: "10,001", 1_234_567: "1,234,567", -12_345: "-12,345",
	} {
		require.Equal(t, want, numfmt.Thousands(n), n)
	}
}

func TestLimitSpan(t *testing.T) {
	t.Parallel()
	for d, want := range map[time.Duration]string{
		2 * time.Hour: "2-hour", time.Hour: "1-hour", 90 * time.Minute: "90-minute",
		3 * time.Hour: "3-hour", 45 * time.Minute: "45-minute", 150 * time.Minute: "150-minute",
		// Not a whole number of minutes: rounded up, at least 1.
		90*time.Minute + 30*time.Second: "91-minute", time.Second: "1-minute",
		30 * time.Second: "1-minute", time.Minute + time.Nanosecond: "2-minute",
	} {
		require.Equal(t, want, numfmt.LimitSpan(d), d.String())
	}
}
