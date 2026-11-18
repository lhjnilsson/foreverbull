package command

import (
	"context"
	"testing"
	"time"

	"github.com/lhjnilsson/foreverbull/internal/stream"
	financeServicePB "github.com/lhjnilsson/foreverbull/pkg/pb/finance/service"
	"github.com/stretchr/testify/require"
)

// The progress line is the agreed copy, word for word, with thousands
// separators.
func TestDownloadProgressText(t *testing.T) {
	require.Equal(t, "Downloading prices: 1,240 of 10,000 symbols", downloadProgressText(1240, 10_000))
	require.Equal(t, "Downloading prices: 0 of 3 symbols", downloadProgressText(0, 3))
}

// Progress starts at 0, reports at most once per interval, and never counts
// past the total.
func TestDownloadProgressReportsAtMostOncePerInterval(t *testing.T) {
	reports := make([]string, 0)
	ctx := stream.WithProgress(context.Background(), func(progress stream.Progress) { reports = append(reports, progress.Text) })
	clock := time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)
	progress := newDownloadProgress(ctx, 3)
	progress.now = func() time.Time { return clock }

	progress.start()
	progress.symbolDone()
	clock = clock.Add(downloadProgressInterval - time.Millisecond)
	progress.symbolDone()
	clock = clock.Add(time.Millisecond)
	progress.symbolDone()
	clock = clock.Add(downloadProgressInterval)
	progress.symbolDone()

	require.Equal(t, []string{
		"Downloading prices: 0 of 3 symbols",
		"Downloading prices: 3 of 3 symbols",
		"Downloading prices: 3 of 3 symbols",
	}, reports)
}

// A provider pause is carried on the progress message, and the next update
// without a pause clears it. The line stays the agreed copy.
func TestDownloadProgressCarriesRateLimit(t *testing.T) {
	reports := make([]stream.Progress, 0)
	ctx := stream.WithProgress(context.Background(), func(progress stream.Progress) {
		reports = append(reports, progress)
	})
	progress := newDownloadProgress(ctx, 10_000)
	progress.done = 1240

	require.NoError(t, progress.noteRateLimit("yahoo", 30))
	progress.symbolDone()

	require.Equal(t, []stream.Progress{
		{
			Text:        "Downloading prices: 1,240 of 10,000 symbols",
			Provider:    "yahoo",
			WaitSeconds: 30,
		},
		{Text: "Downloading prices: 1,241 of 10,000 symbols"},
	}, reports)
}

func TestDownloadProgressRejectsANonPositiveWait(t *testing.T) {
	ctx := stream.WithProgress(context.Background(), func(stream.Progress) {})
	progress := newDownloadProgress(ctx, 1)
	require.ErrorContains(t, progress.noteRateLimit("yahoo", 0), "rate-limit wait of 0 s")
}

// Any provider name is carried. A progress note must not fail the download.
func TestDownloadProgressCarriesAnUnknownProvider(t *testing.T) {
	reports := make([]stream.Progress, 0)
	ctx := stream.WithProgress(context.Background(), func(progress stream.Progress) {
		reports = append(reports, progress)
	})
	progress := newDownloadProgress(ctx, 10)
	require.NoError(t, progress.noteRateLimit("polygon", 30))
	require.Equal(t, []stream.Progress{{
		Text:        "Downloading prices: 0 of 10 symbols",
		Provider:    "polygon",
		WaitSeconds: 30,
	}}, reports)
}

func TestRateLimitPauseFrame(t *testing.T) {
	provider, seconds, ok, err := rateLimitPause(&financeServicePB.DownloadHistoricalDataResponse{
		RateLimit: &financeServicePB.DownloadHistoricalDataResponse_RateLimit{
			Provider: "yahoo", WaitSeconds: 30,
		},
	})
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "yahoo", provider)
	require.Equal(t, int32(30), seconds)

	_, _, ok, err = rateLimitPause(&financeServicePB.DownloadHistoricalDataResponse{
		Symbol: "AAPL",
		Status: financeServicePB.DownloadHistoricalDataResponse_STATUS_OK,
	})
	require.NoError(t, err)
	require.False(t, ok)

	_, _, _, err = rateLimitPause(&financeServicePB.DownloadHistoricalDataResponse{
		Symbol: "AAPL",
		RateLimit: &financeServicePB.DownloadHistoricalDataResponse_RateLimit{
			Provider: "yahoo", WaitSeconds: 30,
		},
	})
	require.EqualError(t, err, "price download sent a rate-limit pause on a symbol result")

	provider, seconds, ok, err = rateLimitPause(&financeServicePB.DownloadHistoricalDataResponse{
		RateLimit: &financeServicePB.DownloadHistoricalDataResponse_RateLimit{
			Provider: "polygon", WaitSeconds: 30,
		},
	})
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "polygon", provider)
	require.Equal(t, int32(30), seconds)

	_, _, _, err = rateLimitPause(&financeServicePB.DownloadHistoricalDataResponse{
		RateLimit: &financeServicePB.DownloadHistoricalDataResponse_RateLimit{
			Provider: "yahoo", WaitSeconds: 0,
		},
	})
	require.EqualError(t, err, "price download named a rate-limit wait of 0 s")
}
