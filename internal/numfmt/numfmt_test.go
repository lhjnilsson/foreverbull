package numfmt_test

import (
	"testing"

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
