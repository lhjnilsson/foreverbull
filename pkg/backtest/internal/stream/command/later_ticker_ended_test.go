package command

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	streampb "github.com/lhjnilsson/foreverbull/internal/stream/pb"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/repository"
	"github.com/stretchr/testify/require"
)

func TestLaterTickerCancelAndTimeout(t *testing.T) {
	t.Parallel()
	const name = "oef-2024q1"
	canceled := "Couldn't load data for backtest 'oef-2024q1': the create was canceled. " +
		"The backtest was partly saved and couldn't be removed. Run `fbull backtest delete oef-2024q1` first. " +
		"Run `fbull backtest create` again."
	timedOut := "Couldn't load data for backtest 'oef-2024q1': the create timed out. " +
		"The backtest was partly saved and couldn't be removed. Run `fbull backtest delete oef-2024q1` first. " +
		"Run `fbull backtest create` again."
	kept := repository.Backtest{Conn: deleteRefused{}}
	gone := repository.Backtest{Conn: deleteOK{}}

	tests := []struct {
		name      string
		cause     error
		backtests repository.Backtest
		status    streampb.State_State
		stored    string
		message   string
		canceled  bool
		deadline  bool
		stayed    bool
	}{
		{
			name: "row kept, cancel", cause: context.Canceled, backtests: kept,
			status: streampb.State_STATE_ERROR, stored: canceled, message: canceled, stayed: true,
		},
		{
			name: "row kept, timeout", cause: context.DeadlineExceeded, backtests: kept,
			status: streampb.State_STATE_ERROR, stored: timedOut, message: timedOut, stayed: true,
		},
		{
			name: "row removed, cancel", cause: context.Canceled, backtests: gone,
			status: streampb.State_STATE_CANCELED, message: "context canceled", canceled: true,
		},
		{
			name: "row removed, timeout", cause: context.DeadlineExceeded, backtests: gone,
			status: streampb.State_STATE_ERROR, stored: "context deadline exceeded",
			message: "context deadline exceeded", deadline: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := discardUnrecordedResolutions(endedContext(t, tc.cause), tc.backtests, name, "id", errors.New("could not store the resolution"))
			state, stored := storedTask(err)
			require.Equal(t, tc.status, state)
			require.Equal(t, tc.stored, stored)
			require.EqualError(t, err, tc.message)
			if tc.canceled {
				require.ErrorIs(t, err, context.Canceled)
			} else {
				require.NotErrorIs(t, err, context.Canceled)
			}
			if tc.deadline {
				require.ErrorIs(t, err, context.DeadlineExceeded)
			} else {
				require.NotErrorIs(t, err, context.DeadlineExceeded)
			}
			require.Equal(t, tc.stayed, rowStayed(err))
			if tc.stayed {
				require.Contains(t, err.Error(), "Run `fbull backtest delete oef-2024q1` first.")
			}
		})
	}
}

// deleteOK is a backtest store whose delete removes the row.
type deleteOK struct{ deleteRefused }

func (deleteOK) Exec(context.Context, string, ...interface{}) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}
