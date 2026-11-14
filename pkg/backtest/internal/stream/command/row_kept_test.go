package command

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	streampb "github.com/lhjnilsson/foreverbull/internal/stream/pb"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/repository"
	"github.com/stretchr/testify/require"
)

// deleteRefused is a backtest store whose delete fails, so the row stays.
type deleteRefused struct{}

func (deleteRefused) Query(context.Context, string, ...interface{}) (pgx.Rows, error) {
	return nil, errors.New("query is not used")
}

func (deleteRefused) QueryRow(context.Context, string, ...interface{}) pgx.Row {
	return nil
}

func (deleteRefused) Exec(context.Context, string, ...interface{}) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, errors.New("backtest delete refused")
}

// endedContext is already canceled or already past its deadline.
func endedContext(t *testing.T, cause error) context.Context {
	t.Helper()
	switch cause {
	case context.DeadlineExceeded:
		ctx, cancel := context.WithDeadline(context.Background(), time.Unix(0, 0))
		t.Cleanup(cancel)
		return ctx
	case context.Canceled:
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		t.Cleanup(cancel)
		return ctx
	default:
		t.Fatalf("cause %v is not a cancel or a timeout", cause)
		return nil
	}
}

// storedTask is what the subscriber writes. context.Canceled is stored as
// canceled with no message, which drops the sentence. Any other error,
// including a deadline, is stored as an error and the message is the error
// text. These sentences have no URL, so that text is stored unchanged.
func storedTask(err error) (streampb.State_State, string) {
	if errors.Is(err, context.Canceled) {
		return streampb.State_STATE_CANCELED, ""
	}
	return streampb.State_STATE_ERROR, err.Error()
}

func TestRowKeptCancelAndTimeoutStoresTheDeleteFirstStep(t *testing.T) {
	t.Parallel()
	const name = "oef-2024q1"
	backtests := repository.Backtest{Conn: deleteRefused{}}
	days := []time.Time{time.Date(2024, 2, 14, 0, 0, 0, 0, time.UTC)}
	gaps := []symbolGaps{{symbol: "XYZ", sessions: []int{0}}}
	canceled := "Couldn't load data for backtest 'oef-2024q1': the create was canceled. " +
		"The backtest was partly saved and couldn't be removed. Run `fbull backtest delete oef-2024q1` first. " +
		"Run `fbull backtest create` again."
	timedOut := "Couldn't load data for backtest 'oef-2024q1': the create timed out. " +
		"The backtest was partly saved and couldn't be removed. Run `fbull backtest delete oef-2024q1` first. " +
		"Run `fbull backtest create` again."
	missing := "Couldn't load data for backtest 'oef-2024q1': yahoo is missing prices for XYZ on 2024-02-14, " +
		"a trading day. The backtest was partly saved and couldn't be removed. Run `fbull backtest delete oef-2024q1` " +
		"first. Try another provider with --provider, or change the backtest dates, then run " +
		"`fbull backtest create` again."

	tests := []struct {
		path    string
		cause   error
		message string
		run     func(context.Context) error
	}{
		{
			path: "generic ingest failure", cause: context.Canceled, message: canceled,
			run: func(ctx context.Context) error {
				return discardFailedIngest(ctx, backtests, name, "id", errors.New("boom"))
			},
		},
		{
			path: "generic ingest failure", cause: context.DeadlineExceeded, message: timedOut,
			run: func(ctx context.Context) error {
				return discardFailedIngest(ctx, backtests, name, "id", errors.New("boom"))
			},
		},
		{
			path: "missing sessions", cause: context.Canceled, message: missing,
			run: func(ctx context.Context) error {
				return discardMissingSessions(ctx, backtests, name, "id", func(removed bool) string {
					return presentMissingSessions(name, "yahoo", days, gaps, removed, nil, 0)
				})
			},
		},
		{
			path: "missing sessions", cause: context.DeadlineExceeded, message: missing,
			run: func(ctx context.Context) error {
				return discardMissingSessions(ctx, backtests, name, "id", func(removed bool) string {
					return presentMissingSessions(name, "yahoo", days, gaps, removed, nil, 0)
				})
			},
		},
		{
			path: "unrecorded cut-off", cause: context.Canceled, message: canceled,
			run: func(ctx context.Context) error {
				return discardUnrecordedCutoff(ctx, backtests, name, "id")
			},
		},
		{
			path: "unrecorded cut-off", cause: context.DeadlineExceeded, message: timedOut,
			run: func(ctx context.Context) error {
				return discardUnrecordedCutoff(ctx, backtests, name, "id")
			},
		},
		{
			path: "unusable cut-off", cause: context.Canceled, message: canceled,
			run: func(ctx context.Context) error {
				return discardUnusableCutoffRow(ctx, backtests, name, "id", errors.New("the recorded session cut-off cannot be used"))
			},
		},
		{
			path: "unusable cut-off", cause: context.DeadlineExceeded, message: timedOut,
			run: func(ctx context.Context) error {
				return discardUnusableCutoffRow(ctx, backtests, name, "id", errors.New("the recorded session cut-off cannot be used"))
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.path+"/"+tc.cause.Error(), func(t *testing.T) {
			t.Parallel()
			err := tc.run(endedContext(t, tc.cause))
			require.NotErrorIs(t, err, context.Canceled)
			require.NotErrorIs(t, err, context.DeadlineExceeded)
			require.True(t, rowStayed(err))
			state, stored := storedTask(err)
			require.Equal(t, streampb.State_STATE_ERROR, state)
			require.Equal(t, tc.message, stored)
			require.EqualError(t, err, tc.message)
			require.Contains(t, err.Error(), "Run `fbull backtest delete oef-2024q1` first.")
		})
	}
}
