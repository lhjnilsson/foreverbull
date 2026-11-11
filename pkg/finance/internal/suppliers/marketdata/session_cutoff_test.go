package marketdata

import (
	"context"
	"testing"
	"time"

	pb "github.com/lhjnilsson/foreverbull/pkg/pb/finance"
	"github.com/stretchr/testify/require"
)

// flippingDownload is a provider clock that has already crossed 16:30, the
// way a fetch reads the clock after the broker's earlier reading. calls
// counts how often the provider asks for it.
func flippingDownload(t *testing.T, calls *int) func() time.Time {
	t.Helper()
	after := newYork(t, "2026-10-30 16:30")
	return func() time.Time {
		*calls++
		return after
	}
}

func sessionDays(bars []*pb.OHLC) []string {
	days := make([]string, 0, len(bars))
	for _, bar := range bars {
		days = append(days, bar.GetTimestamp().AsTime().UTC().Format(time.DateOnly))
	}
	return days
}

// The broker's cutoff is 16:30 New York on the previous session. A provider
// clock that has since crossed 16:30 must not store that new session, and
// must not be read to decide the cap. Without the cutoff the same clock
// stores it. The previous session's bar is still stored: the end date is
// inclusive.
func TestProvidersKeepTheBrokerCutoffWhenTheClockClosesMidDownload(t *testing.T) {
	t.Parallel()
	yesterday := time.Date(2026, 10, 29, 0, 0, 0, 0, time.UTC)
	today := time.Date(2026, 10, 30, 0, 0, 0, 0, time.UTC)
	require.Equal(t, yesterday, LastCompletedSessionDate(SessionAsOf(yesterday)))

	ctx := WithSessionAsOf(context.Background(), SessionAsOf(yesterday))
	require.Contains(t, UnclosedSessionsNotice(today, today), "2026-10-30 isn't included")
	start := yesterday
	// The uncapped end is today, which is what a provider used to be asked
	// for after the broker had already capped the notice at yesterday.
	end := today

	t.Run("yahoo", func(t *testing.T) {
		t.Parallel()
		var reads int
		yesterdayOpen := newYork(t, "2026-10-29 09:30")
		todayOpen := newYork(t, "2026-10-30 09:30")
		client := &YahooClient{
			chartURL: liveBarServer(t, yesterdayOpen, todayOpen, "10.4"),
			now:      flippingDownload(t, &reads),
		}
		bars, err := client.GetOHLC(ctx, "xyz", start, &end)
		require.NoError(t, err)
		require.Equal(t, []string{"2026-10-29"}, sessionDays(bars))
		require.Zero(t, reads)

		open := &YahooClient{
			chartURL: liveBarServer(t, yesterdayOpen, todayOpen, "10.4"),
			now:      flippingDownload(t, new(int)),
		}
		kept, err := open.GetOHLC(context.Background(), "xyz", start, &end)
		require.NoError(t, err)
		require.Equal(t, []string{"2026-10-29", "2026-10-30"}, sessionDays(kept))
	})

	t.Run("stooq", func(t *testing.T) {
		t.Parallel()
		var reads int
		client, asked := stooqSessionServer(t, newYork(t, "2026-10-30 11:00"))
		client.now = flippingDownload(t, &reads)
		bars, err := client.GetOHLC(ctx, "AAPL", start, &end)
		require.NoError(t, err)
		require.Equal(t, []string{"2026-10-29"}, sessionDays(bars))
		require.Equal(t, []string{"20261029"}, *asked)
		require.Zero(t, reads)

		open, openAsked := stooqSessionServer(t, newYork(t, "2026-10-30 16:30"))
		kept, err := open.GetOHLC(context.Background(), "AAPL", start, &end)
		require.NoError(t, err)
		require.Equal(t, []string{"2026-10-29", "2026-10-30"}, sessionDays(kept))
		require.Equal(t, []string{"20261030"}, *openAsked)
	})

	t.Run("alpaca", func(t *testing.T) {
		t.Parallel()
		var reads int
		yesterdayBar := newYork(t, "2026-10-29 00:00")
		todayBar := newYork(t, "2026-10-30 00:00")
		bars, _ := alpacaRangeServer(t, yesterdayBar, todayBar)
		bars.now = flippingDownload(t, &reads)
		ohlcs, err := bars.GetOHLC(ctx, "XYZ", start, &end)
		require.NoError(t, err)
		require.Equal(t, []string{"2026-10-29"}, sessionDays(ohlcs))
		require.True(t, bars.NothingToFetch(ctx, today, nil))
		require.Zero(t, reads)

		open, _ := alpacaRangeServer(t, yesterdayBar, todayBar)
		open.now = func() time.Time { return newYork(t, "2026-10-30 16:30") }
		kept, err := open.GetOHLC(context.Background(), "XYZ", start, &end)
		require.NoError(t, err)
		require.Equal(t, []string{"2026-10-29", "2026-10-30"}, sessionDays(kept))
		require.False(t, open.NothingToFetch(context.Background(), today, nil))
	})
}
