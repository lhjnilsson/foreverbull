package command

import (
	"context"
	"testing"
	"time"

	"github.com/lhjnilsson/foreverbull/internal/stream"
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
	ctx := stream.WithProgress(context.Background(), func(text string) { reports = append(reports, text) })
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
