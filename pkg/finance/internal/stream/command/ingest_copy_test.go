package command

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// The ingest's time-limit sentence names whole hours in hours and anything
// else in minutes, word for word.
func TestIngestTimeLimitCopy(t *testing.T) {
	t.Parallel()
	for limit, want := range map[time.Duration]string{
		2 * time.Hour: "The price import hit the 2-hour limit at AAPL. Prices imported before AAPL were kept. " +
			"Import fewer symbols at a time, or try again later.",
		time.Hour: "The price import hit the 1-hour limit at AAPL. Prices imported before AAPL were kept. " +
			"Import fewer symbols at a time, or try again later.",
		90 * time.Minute: "The price import hit the 90-minute limit at AAPL. Prices imported before AAPL were kept. " +
			"Import fewer symbols at a time, or try again later.",
	} {
		require.EqualError(t, ingestTimeLimitError(limit, "AAPL"), want, limit.String())
	}
}
