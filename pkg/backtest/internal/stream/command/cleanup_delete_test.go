package command

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/lhjnilsson/foreverbull/internal/stream"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/engine"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/repository"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/stream/dependency"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// cleanupClock is Saturday 2026-10-31 01:00 New York, before that day's
// close, so the last completed session is 2026-10-30. A 2024 period is
// wholly before it, and a 2027 period is wholly after it.
func cleanupClock(t *testing.T) {
	t.Helper()
	restore := SetSessionClock(func() time.Time {
		return time.Date(2026, 10, 31, 5, 0, 0, 0, time.UTC)
	})
	t.Cleanup(restore)
}

func engineMessage(eng engine.Engine, callErr error) *stream.MockMessage {
	msg := new(stream.MockMessage)
	if callErr != nil {
		msg.On("Call", mock.Anything, dependency.GetEngineKey).Return(nil, callErr)
		return msg
	}
	msg.On("Call", mock.Anything, dependency.GetEngineKey).Return(eng, nil)
	return msg
}

// causeHas reports whether a wrapped error, not the user sentence, contains text.
func causeHas(err error, text string) bool {
	if err == nil {
		return false
	}
	if multi, ok := err.(interface{ Unwrap() []error }); ok {
		for _, next := range multi.Unwrap() {
			if next != nil && next.Error() != err.Error() && strings.Contains(next.Error(), text) {
				return true
			}
			if causeHas(next, text) {
				return true
			}
		}
		return false
	}
	next := errors.Unwrap(err)
	if next == nil {
		return false
	}
	if next.Error() != err.Error() && strings.Contains(next.Error(), text) {
		return true
	}
	return causeHas(next, text)
}

// The calendar cannot be read, so the start check stores the row and the
// download step removes it. A failed delete says to delete first. A delete
// that works keeps the sentence the download step already used.
func TestCalendarUnreadableSaysToDeleteFirstWhenCleanupFails(t *testing.T) {
	cleanupClock(t)
	const (
		failed = "Couldn't load data for backtest 'oef-2024q1': the data service failed while ingesting prices. " +
			"The backtest was partly saved and couldn't be removed. Run `fbull backtest delete oef-2024q1` first. " +
			"Run `fbull backtest create` again once it's fixed.\n" +
			"Details: error getting zipline engine: pod not ready"
		removed = "Couldn't load data for backtest 'oef-2024q1': the data service failed while ingesting prices. " +
			"Nothing was saved, so run `fbull backtest create` again once it's fixed.\n" +
			"Details: error getting zipline engine: pod not ready"
	)
	start := time.Date(1989, 12, 31, 0, 0, 0, 0, time.UTC)
	end := time.Date(1990, 6, 29, 0, 0, 0, 0, time.UTC)

	_, err := readClosedPeriod(context.Background(), engineMessage(nil, errors.New("pod not ready")),
		repository.Backtest{Conn: deleteRefused{}}, "oef-2024q1", "id", start, end)
	require.EqualError(t, err, failed)
	require.NotContains(t, err.Error(), "backtest delete refused")
	require.NotContains(t, err.Error(), "failed to delete backtest")
	require.True(t, causeHas(err, "backtest delete refused"))
	require.True(t, rowStayed(err))

	_, err = readClosedPeriod(context.Background(), engineMessage(nil, errors.New("pod not ready")),
		repository.Backtest{Conn: deleteOK{}}, "oef-2024q1", "id", start, end)
	require.EqualError(t, err, removed)
	require.False(t, causeHas(err, "backtest delete refused"))
	require.False(t, rowStayed(err))
}

// An older Zipline image does not answer OUT_OF_RANGE. The start check cannot
// name the first session, the row is stored, and the download step removes it.
func TestOldCalendarImageSaysToDeleteFirstWhenCleanupFails(t *testing.T) {
	cleanupClock(t)
	const (
		failed = "Couldn't load data for backtest 'oef-2024q1': the data service failed while ingesting prices. " +
			"The backtest was partly saved and couldn't be removed. Run `fbull backtest delete oef-2024q1` first. " +
			"Run `fbull backtest create` again once it's fixed.\n" +
			"Details: Exception calling application: DateOutOfBounds"
		removed = "Couldn't load data for backtest 'oef-2024q1': the data service failed while ingesting prices. " +
			"Nothing was saved, so run `fbull backtest create` again once it's fixed.\n" +
			"Details: Exception calling application: DateOutOfBounds"
	)
	start := time.Date(1989, 12, 31, 0, 0, 0, 0, time.UTC)
	end := time.Date(1990, 6, 29, 0, 0, 0, 0, time.UTC)
	cause := fmt.Errorf("error listing XNYS sessions: %w",
		status.Error(codes.Unknown, "Exception calling application: DateOutOfBounds"))
	eng := new(engine.MockEngine)
	eng.On("Sessions", mock.Anything, start, end).Return(nil, cause)

	_, err := readClosedPeriod(context.Background(), engineMessage(eng, nil),
		repository.Backtest{Conn: deleteRefused{}}, "oef-2024q1", "id", start, end)
	require.EqualError(t, err, failed)
	require.NotContains(t, err.Error(), "backtest delete refused")
	require.True(t, causeHas(err, "backtest delete refused"))
	require.True(t, rowStayed(err))

	_, err = readClosedPeriod(context.Background(), engineMessage(eng, nil),
		repository.Backtest{Conn: deleteOK{}}, "oef-2024q1", "id", start, end)
	require.EqualError(t, err, removed)
	require.False(t, rowStayed(err))
}

// A start after the calendar's last session is stored, then removed by the
// download step. A failed delete keeps that step and says to delete first.
func TestStartAfterLastSessionSaysToDeleteFirstWhenCleanupFails(t *testing.T) {
	cleanupClock(t)
	const (
		failed = "Couldn't load data for backtest 'oef-2024q1': it ends on 2027-12-12, after the last trading " +
			"day in foreverbull's market calendar (2027-11-02). The backtest was partly saved and couldn't be " +
			"removed. Run `fbull backtest delete oef-2024q1` first. Choose an end on or before 2027-11-02, then " +
			"run `fbull backtest create` again."
		removed = "Couldn't load data for backtest 'oef-2024q1': it ends on 2027-12-12, after the last trading " +
			"day in foreverbull's market calendar (2027-11-02). The backtest wasn't saved. Choose an end on or " +
			"before 2027-11-02, then run `fbull backtest create` again."
		canceled = "Couldn't load data for backtest 'oef-2024q1': the create was canceled. " +
			"The backtest was partly saved and couldn't be removed. Run `fbull backtest delete oef-2024q1` first. " +
			"Run `fbull backtest create` again."
	)
	start := time.Date(2027, 12, 2, 0, 0, 0, 0, time.UTC)
	end := time.Date(2027, 12, 12, 0, 0, 0, 0, time.UTC)
	eng := new(engine.MockEngine)
	eng.On("Sessions", mock.Anything, end, end).Return(nil, &engine.CalendarEndError{
		Calendar: "XNYS", LastSession: time.Date(2027, 11, 2, 0, 0, 0, 0, time.UTC),
	})

	_, err := readClosedPeriod(context.Background(), engineMessage(eng, nil),
		repository.Backtest{Conn: deleteRefused{}}, "oef-2024q1", "id", start, end)
	require.EqualError(t, err, failed)
	require.NotContains(t, err.Error(), "backtest delete refused")
	require.NotContains(t, err.Error(), "data service")
	require.True(t, causeHas(err, "backtest delete refused"))
	require.True(t, rowStayed(err))

	_, err = readClosedPeriod(context.Background(), engineMessage(eng, nil),
		repository.Backtest{Conn: deleteOK{}}, "oef-2024q1", "id", start, end)
	require.EqualError(t, err, removed)
	require.False(t, causeHas(err, "backtest delete refused"))
	require.False(t, rowStayed(err))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = readClosedPeriod(ctx, engineMessage(eng, nil),
		repository.Backtest{Conn: deleteRefused{}}, "oef-2024q1", "id", start, end)
	require.EqualError(t, err, canceled)
	require.True(t, causeHas(err, "backtest delete refused"))
	require.NotErrorIs(t, err, context.Canceled)

	_, err = readClosedPeriod(ctx, engineMessage(eng, nil),
		repository.Backtest{Conn: deleteOK{}}, "oef-2024q1", "id", start, end)
	require.Equal(t, ctx.Err(), err)
}
