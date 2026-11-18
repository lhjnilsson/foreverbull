package stream

import (
	"context"
	"fmt"

	"github.com/lhjnilsson/foreverbull/internal/stream/pb"
	"google.golang.org/protobuf/proto"
)

type progressKey struct{}

// Progress is one progress report. Text is the line. Provider and
// WaitSeconds are set only while a price download is paused for the
// provider's rate limit. The next report leaves them empty, which clears
// the note.
type Progress struct {
	Text        string
	Provider    string
	WaitSeconds int32
}

// ReportProgress sends text as the running command's progress, for example
// "Downloading prices: 1,240 of 10,000 symbols". The subscriber publishes it
// over core NATS on TOPIC_TASK_PROGRESS, WaitForCompletion hands it to its
// progress callback, and RunOrchestration emits it as a RUNNING step whose
// message is the text. Progress is not kept on the JetStream stream and is
// not a task state. A ctx from outside a subscriber has no reporter, and the
// call does nothing. The caller still throttles how often it reports.
func ReportProgress(ctx context.Context, text string) {
	reportProgress(ctx, Progress{Text: text})
}

// ReportRateLimit sends text with the provider pause the download is waiting
// out. WaitSeconds is the whole seconds of that pause.
func ReportRateLimit(ctx context.Context, text, provider string, waitSeconds int32) {
	reportProgress(ctx, Progress{Text: text, Provider: provider, WaitSeconds: waitSeconds})
}

func reportProgress(ctx context.Context, progress Progress) {
	if report, ok := ctx.Value(progressKey{}).(func(Progress)); ok {
		report(progress)
	}
}

// WithProgress returns ctx with report as its progress reporter. The
// subscriber sets it for every command; a test sets it to read the reports.
func WithProgress(ctx context.Context, report func(progress Progress)) context.Context {
	return context.WithValue(ctx, progressKey{}, report)
}

func marshalProgress(progress Progress) ([]byte, error) {
	payload, err := proto.Marshal(&pb.Progress{
		Text:                 progress.Text,
		RateLimitProvider:    progress.Provider,
		RateLimitWaitSeconds: progress.WaitSeconds,
	})
	if err != nil {
		return nil, fmt.Errorf("error encoding progress: %w", err)
	}
	return payload, nil
}

func unmarshalProgress(payload []byte) (Progress, error) {
	var wire pb.Progress
	if err := proto.Unmarshal(payload, &wire); err != nil {
		return Progress{}, fmt.Errorf("error reading progress: %w", err)
	}
	return Progress{
		Text:        wire.GetText(),
		Provider:    wire.GetRateLimitProvider(),
		WaitSeconds: wire.GetRateLimitWaitSeconds(),
	}, nil
}
