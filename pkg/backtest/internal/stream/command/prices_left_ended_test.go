package command

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/lhjnilsson/foreverbull/internal/storage"
	streampb "github.com/lhjnilsson/foreverbull/internal/stream/pb"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/repository"
	"github.com/minio/minio-go/v7"
	"github.com/stretchr/testify/require"
)

// priceDeleteRefused is an object store whose price delete fails.
type priceDeleteRefused struct{}

func (priceDeleteRefused) ListObjects(context.Context, storage.Bucket) (*[]storage.Object, error) {
	return nil, errors.New("list is not used")
}

func (priceDeleteRefused) GetObject(context.Context, storage.Bucket, string) (*storage.Object, error) {
	return nil, errors.New("get is not used")
}

func (priceDeleteRefused) CreateObject(context.Context, storage.Bucket, string, ...func(*minio.PutObjectOptions) error) (*storage.Object, error) {
	return nil, errors.New("create is not used")
}

func (priceDeleteRefused) DeleteObject(context.Context, storage.Bucket, string) error {
	return errPriceDelete
}

var errPriceDelete = errors.New("object storage refused the delete")

func TestPricesLeftCancelAndTimeout(t *testing.T) {
	t.Parallel()
	const name = "oef-2024q1"
	keptCancel := "Couldn't load data for backtest 'oef-2024q1': the create was canceled. " +
		"The backtest was partly saved and couldn't be removed, and some of its stored prices couldn't be removed either. " +
		"Run `fbull backtest delete oef-2024q1` first. Run `fbull backtest create` again."
	keptTimeout := "Couldn't load data for backtest 'oef-2024q1': the create timed out. " +
		"The backtest was partly saved and couldn't be removed, and some of its stored prices couldn't be removed either. " +
		"Run `fbull backtest delete oef-2024q1` first. Run `fbull backtest create` again."
	goneCancel := "Couldn't load data for backtest 'oef-2024q1': the create was canceled. " +
		"The backtest wasn't saved, but some of its prices couldn't be removed. They don't stop a new create. " +
		"Run `fbull backtest create` again."
	goneTimeout := "Couldn't load data for backtest 'oef-2024q1': the create timed out. " +
		"The backtest wasn't saved, but some of its prices couldn't be removed. They don't stop a new create. " +
		"Run `fbull backtest create` again."
	days := []time.Time{time.Date(2024, 2, 14, 0, 0, 0, 0, time.UTC)}
	gaps := []symbolGaps{{symbol: "XYZ", sessions: []int{0}}}
	missingKept := "Couldn't load data for backtest 'oef-2024q1': yahoo is missing prices for XYZ on 2024-02-14, " +
		"a trading day. The backtest was partly saved and couldn't be removed, and some of its stored prices " +
		"couldn't be removed either. Run `fbull backtest delete oef-2024q1` first. Try another provider with " +
		"--provider, or change the backtest dates, then run `fbull backtest create` again."
	missingGone := "Couldn't load data for backtest 'oef-2024q1': yahoo is missing prices for XYZ on 2024-02-14, " +
		"a trading day. The backtest wasn't saved, but some of its prices couldn't be removed. They don't stop a " +
		"new create. Try another provider with --provider, or change the backtest dates, then run " +
		"`fbull backtest create` again."
	kept := repository.Backtest{Conn: deleteRefused{}}
	gone := repository.Backtest{Conn: deleteOK{}}
	store := priceDeleteRefused{}

	tests := []struct {
		name     string
		cause    error
		message  string
		status   streampb.State_State
		stored   string
		canceled bool
		deadline bool
		run      func(context.Context) error
	}{
		{
			name: "unrecorded cut-off, row kept, cancel", cause: context.Canceled,
			message: keptCancel, status: streampb.State_STATE_ERROR, stored: keptCancel,
			run: func(ctx context.Context) error {
				return cleanupStoredPrices(ctx, store, name, name, func() error {
					return discardUnrecordedCutoff(ctx, kept, name, "id")
				})
			},
		},
		{
			name: "unrecorded cut-off, row removed, cancel", cause: context.Canceled,
			message: goneCancel, status: streampb.State_STATE_CANCELED, canceled: true,
			run: func(ctx context.Context) error {
				return cleanupStoredPrices(ctx, store, name, name, func() error {
					return discardUnrecordedCutoff(ctx, gone, name, "id")
				})
			},
		},
		{
			name: "unrecorded cut-off, row removed, timeout", cause: context.DeadlineExceeded,
			message: goneTimeout, status: streampb.State_STATE_ERROR, stored: goneTimeout, deadline: true,
			run: func(ctx context.Context) error {
				return cleanupStoredPrices(ctx, store, name, name, func() error {
					return discardUnrecordedCutoff(ctx, gone, name, "id")
				})
			},
		},
		{
			name: "unusable cut-off, row kept, cancel", cause: context.Canceled,
			message: keptCancel, status: streampb.State_STATE_ERROR, stored: keptCancel,
			run: func(ctx context.Context) error {
				return discardUnusableCutoff(ctx, store, name, kept, name, "id", errors.New("cut-off cannot be used"))
			},
		},
		{
			name: "unusable cut-off, row kept, timeout", cause: context.DeadlineExceeded,
			message: keptTimeout, status: streampb.State_STATE_ERROR, stored: keptTimeout,
			run: func(ctx context.Context) error {
				return discardUnusableCutoff(ctx, store, name, kept, name, "id", errors.New("cut-off cannot be used"))
			},
		},
		{
			name: "unusable cut-off, row removed, cancel", cause: context.Canceled,
			message: goneCancel, status: streampb.State_STATE_CANCELED, canceled: true,
			run: func(ctx context.Context) error {
				return discardUnusableCutoff(ctx, store, name, gone, name, "id", errors.New("cut-off cannot be used"))
			},
		},
		{
			name: "unusable cut-off, row removed, timeout", cause: context.DeadlineExceeded,
			message: goneTimeout, status: streampb.State_STATE_ERROR, stored: goneTimeout, deadline: true,
			run: func(ctx context.Context) error {
				return discardUnusableCutoff(ctx, store, name, gone, name, "id", errors.New("cut-off cannot be used"))
			},
		},
		{
			name: "missing sessions, row kept, cancel", cause: context.Canceled,
			message: missingKept, status: streampb.State_STATE_ERROR, stored: missingKept,
			run: func(ctx context.Context) error {
				return cleanupStoredPrices(ctx, store, name, name, func() error {
					return discardMissingSessions(ctx, kept, name, "id", func(removed bool) string {
						return presentMissingSessions(name, "yahoo", days, gaps, removed, nil, 0)
					})
				})
			},
		},
		{
			name: "missing sessions, row kept, timeout", cause: context.DeadlineExceeded,
			message: missingKept, status: streampb.State_STATE_ERROR, stored: missingKept,
			run: func(ctx context.Context) error {
				return cleanupStoredPrices(ctx, store, name, name, func() error {
					return discardMissingSessions(ctx, kept, name, "id", func(removed bool) string {
						return presentMissingSessions(name, "yahoo", days, gaps, removed, nil, 0)
					})
				})
			},
		},
		{
			name: "missing sessions, row removed, cancel", cause: context.Canceled,
			message: missingGone, status: streampb.State_STATE_ERROR, stored: missingGone,
			run: func(ctx context.Context) error {
				return cleanupStoredPrices(ctx, store, name, name, func() error {
					return discardMissingSessions(ctx, gone, name, "id", func(removed bool) string {
						return presentMissingSessions(name, "yahoo", days, gaps, removed, nil, 0)
					})
				})
			},
		},
		{
			name: "missing sessions, row removed, timeout", cause: context.DeadlineExceeded,
			message: missingGone, status: streampb.State_STATE_ERROR, stored: missingGone,
			run: func(ctx context.Context) error {
				return cleanupStoredPrices(ctx, store, name, name, func() error {
					return discardMissingSessions(ctx, gone, name, "id", func(removed bool) string {
						return presentMissingSessions(name, "yahoo", days, gaps, removed, nil, 0)
					})
				})
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := tc.run(endedContext(t, tc.cause))
			state, stored := storedTask(err)
			require.Equal(t, tc.status, state)
			require.Equal(t, tc.stored, stored)
			require.EqualError(t, err, tc.message)
			require.ErrorIs(t, err, errPriceDelete)
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
		})
	}
}

func TestRowKeptShapeReportsTheCleanupError(t *testing.T) {
	t.Parallel()
	err := cleanupStoredPrices(context.Background(), priceDeleteRefused{}, "demo", "oef-2024q1", func() error {
		return markKept(false, plainSentence("the row stayed without the not-removed sentence"))
	})
	require.EqualError(t, err, "Couldn't load data for backtest 'oef-2024q1': foreverbull hit an internal error while cleaning up. "+
		"The backtest was partly saved and couldn't be removed, and some of its stored prices couldn't be removed either. "+
		"Run `fbull backtest delete oef-2024q1` first, then run `fbull backtest create` again. "+
		"Please report it at https://github.com/lhjnilsson/foreverbull/issues with this message.")
	require.ErrorIs(t, err, errRowKeptShape)
	require.ErrorIs(t, err, errPriceDelete)
	require.NotContains(t, err.Error(), "row-kept message has no not-removed sentence")
}
