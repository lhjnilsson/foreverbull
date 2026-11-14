package command

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/internal/storage"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/repository"
	"github.com/lhjnilsson/foreverbull/pkg/pb"
	"github.com/lhjnilsson/foreverbull/pkg/pb/backtest"
	"github.com/rs/zerolog/log"
	"google.golang.org/protobuf/proto"
)

// recordSessionCutoff stores the capped period end from the one clock
// reading, so ingest builds the bundle through that date and does not read
// the clock again. A failed write stops the download: there is no later
// guess.
func recordSessionCutoff(ctx context.Context, backtests repository.Backtest, name, id string, end time.Time) error {
	var err error
	if environment.GetAuthEnabled() {
		err = backtests.SaveClosedEndByID(ctx, id, end)
	} else {
		err = backtests.SaveClosedEnd(ctx, name, end)
	}
	if err != nil {
		log.Warn().Err(err).Str("backtest", name).Msg("could not record the session cut-off")
		return discardUnrecordedCutoff(ctx, backtests, name, id)
	}
	return nil
}

// loadSessionCutoff reads the cut-off the download recorded. A missing one
// fails ingest: the bundle is not built through a new clock reading or the
// uncapped requested end.
func loadSessionCutoff(ctx context.Context, store storage.Storage, key string, backtests repository.Backtest, name, id string) (time.Time, error) {
	var (
		closed time.Time
		err    error
	)
	if environment.GetAuthEnabled() {
		closed, err = backtests.ClosedEndByID(ctx, id)
	} else {
		closed, err = backtests.ClosedEnd(ctx, name)
	}
	if err != nil {
		if errors.Is(err, repository.ErrNoSessionCutoff) {
			return time.Time{}, cleanupStoredPrices(ctx, store, key, name, func() error {
				return discardUnrecordedCutoff(ctx, backtests, name, id)
			})
		}
		return time.Time{}, finishFailedIngest(ctx, store, key, backtests, name, id, fmt.Errorf("read session cut-off: %w", err))
	}
	return closed, nil
}

func discardUnrecordedCutoff(ctx context.Context, backtests repository.Backtest, name, id string) error {
	err := deleteAfterFailure(ctx, backtests, name, id)
	removed := err == nil || errors.Is(err, pgx.ErrNoRows)
	if !removed {
		log.Warn().Err(err).Str("backtest", name).Msg("error deleting backtest with no session cut-off")
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		return contextKept(name, removed, ctxErr)
	}
	saved, next := savedAndNext(name, "Run `fbull backtest create` again.", removed)
	return markKept(removed, plainSentence(fmt.Sprintf("Couldn't load data for backtest '%s': foreverbull didn't record which trading "+
		"session it stops at. %s %s", name, saved, next)))
}

func discardUnusableCutoff(ctx context.Context, store storage.Storage, key string, backtests repository.Backtest, name, id string, cause error) error {
	return cleanupStoredPrices(ctx, store, key, name, func() error {
		return discardUnusableCutoffRow(ctx, backtests, name, id, cause)
	})
}

func discardUnusableCutoffRow(ctx context.Context, backtests repository.Backtest, name, id string, cause error) error {
	log.Warn().Err(cause).Str("backtest", name).Msg("session cut-off cannot be used")
	err := deleteAfterFailure(ctx, backtests, name, id)
	removed := err == nil || errors.Is(err, pgx.ErrNoRows)
	if !removed {
		log.Warn().Err(err).Str("backtest", name).Msg("error deleting backtest with an unusable session cut-off")
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		return contextKept(name, removed, ctxErr)
	}
	saved, next := savedAndNext(name, "Run `fbull backtest create` again.", removed)
	return markKept(removed, plainSentence(fmt.Sprintf("Couldn't load data for backtest '%s': %s. %s %s", name, cause.Error(), saved, next)))
}

// backtestThroughCutoff is a copy of stored whose periods end on or before
// closed, the capped end the download recorded. The stored dates stay as
// requested. A cut-off after the backtest end, or a period that starts
// after the cut-off, cannot be honored.
func backtestThroughCutoff(stored *backtest.Backtest, closed time.Time) (*backtest.Backtest, error) {
	periods := stored.GetPeriods()
	if len(periods) == 0 || periods[len(periods)-1].GetEndDate() == nil {
		return nil, errors.New("the backtest has no end date")
	}
	closed = utcDate(closed)
	last := pb.DateToTime(periods[len(periods)-1].GetEndDate())
	if closed.After(last) {
		return nil, fmt.Errorf("the recorded session cut-off %s is after the backtest ends on %s",
			closed.Format(time.DateOnly), last.Format(time.DateOnly))
	}
	out, isBacktest := proto.Clone(stored).(*backtest.Backtest)
	if !isBacktest {
		return nil, errors.New("the backtest could not be copied")
	}
	capped := pb.GoTimeToDate(closed)
	for _, period := range out.GetPeriods() {
		if period.GetStartDate() == nil || period.GetEndDate() == nil {
			return nil, errors.New("a period has no dates")
		}
		start := pb.DateToTime(period.GetStartDate())
		end := pb.DateToTime(period.GetEndDate())
		if start.After(closed) {
			return nil, fmt.Errorf("a period starts on %s, after the session cut-off %s",
				start.Format(time.DateOnly), closed.Format(time.DateOnly))
		}
		if end.After(closed) {
			period.EndDate = capped
		}
	}
	return out, nil
}

// contextKept returns ctxErr when the row was removed, so a cleaned-up
// cancel still reads as canceled. When the row stayed, the sentence names
// the cancel or the timeout and says to delete the backtest first. The
// context error is not wrapped: the subscriber stores a cancel with no
// message when errors.Is(context.Canceled), which would hide that step.
func contextKept(name string, removed bool, ctxErr error) error {
	if removed {
		return ctxErr
	}
	return markKept(false, plainSentence(endedIngestMessage(name, ctxErr)))
}

func utcDate(t time.Time) time.Time {
	year, month, day := t.UTC().Date()
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}
