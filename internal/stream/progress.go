package stream

import "context"

type progressKey struct{}

// ReportProgress sends text as the running command's progress, for example
// "Downloading prices: 1,240 of 10,000 symbols". The subscriber publishes it
// over core NATS on TOPIC_TASK_PROGRESS, WaitForCompletion hands it to its
// progress callback, and RunOrchestration emits it as a RUNNING step whose
// message is the text. Progress is not kept on the JetStream stream and is
// not a task state. A ctx from outside a subscriber has no reporter, and the
// call does nothing. The caller still throttles how often it reports.
func ReportProgress(ctx context.Context, text string) {
	if report, ok := ctx.Value(progressKey{}).(func(string)); ok {
		report(text)
	}
}

// WithProgress returns ctx with report as its progress reporter. The
// subscriber sets it for every command; a test sets it to read the reports.
func WithProgress(ctx context.Context, report func(text string)) context.Context {
	return context.WithValue(ctx, progressKey{}, report)
}
