package api

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/lhjnilsson/foreverbull/internal/stream"
	internalPb "github.com/lhjnilsson/foreverbull/internal/stream/pb"
	"github.com/lhjnilsson/foreverbull/internal/usercopy"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/repository"
	"github.com/rs/zerolog/log"
)

// queueFullCleanupTimeout bounds the delete after a create failed on a full
// task queue. It runs on a context that outlives a client that left.
const queueFullCleanupTimeout = 10 * time.Second

// createdBacktest is the backtest a create stores. exists is by name, or by
// owner and name when auth is on, and answers whether that name was stored
// before this create ran. remove deletes the row this create stored and
// returns pgx.ErrNoRows when that row is not there.
type createdBacktest struct {
	name   string
	exists func(context.Context) (bool, error)
	remove func(context.Context) error
}

// queueFullCreate follows a create's orchestration for a step that failed on
// a full task queue (stream.StepNotRecorded). The worker never reached its
// own cleanup then: a refused COMPLETED leaves what the step stored. Only a
// backtest that didn't exist before this create is this create's own, so
// only then is it removed. The failed step's message becomes the whole
// sentence, which the client prints as the error.
//
// The backtest is removed by the id this create stored, recorded under the
// CREATING task's message id. A second create of the same name fails with
// repository.ErrBacktestNameTaken and stores nothing, so it has no row to
// remove. A CREATING task the queue refused before it ran also stored nothing.
type queueFullCreate struct {
	backtest      createdBacktest
	existedBefore bool
}

// newQueueFullCreate reads whether the backtest exists before the create
// runs. A failed read fails the create before anything is sent.
func newQueueFullCreate(ctx context.Context, backtest createdBacktest) (*queueFullCreate, error) {
	existed, err := backtest.exists(ctx)
	if err != nil {
		return nil, fmt.Errorf("error checking backtest %s before create: %w", backtest.name, err)
	}
	return &queueFullCreate{backtest: backtest, existedBefore: existed}, nil
}

// rewrite replaces a full-queue step's bare cause with the create's
// sentence, after removing what this create stored. Other steps pass
// through unchanged.
func (q *queueFullCreate) rewrite(ctx context.Context, step *internalPb.Orchestration_Step) {
	if !stream.StepNotRecorded(step) {
		return
	}
	sentence := usercopy.BacktestCreateQueueFull(q.backtest.name, q.removeOwn(ctx))
	step.Error = &sentence
}

// removeOwn deletes the backtest when this create stored it, and reports
// whether nothing of this create is left. A delete that fails is logged
// and reported as not removed, so the sentence says to delete it first.
func (q *queueFullCreate) removeOwn(ctx context.Context) bool {
	if q.existedBefore {
		// The create was refused before it stored anything: a stored
		// backtest of the same name makes the create itself fail.
		return true
	}
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), queueFullCleanupTimeout)
	defer cancel()
	err := q.backtest.remove(cleanupCtx)
	if err == nil || errors.Is(err, pgx.ErrNoRows) {
		return true
	}
	log.Error().Err(err).Str("backtest", q.backtest.name).Msg("error deleting backtest after its create failed on a full task queue")
	return false
}

// namedBacktest is the created backtest by name, with auth off.
// remove deletes the row this create's task stored, not the name.
func namedBacktest(backtests repository.Backtest, name, messageID string) createdBacktest {
	return createdBacktest{
		name: name,
		exists: func(ctx context.Context) (bool, error) {
			_, err := backtests.Get(ctx, name)
			if errors.Is(err, pgx.ErrNoRows) {
				return false, nil
			}
			return err == nil, err
		},
		remove: removeStoredBacktest(backtests, messageID),
	}
}

// ownedBacktest is the created backtest by owner and name, with auth on.
// remove deletes the row this create's task stored, not the name.
func ownedBacktest(backtests repository.Backtest, ownerID, name, messageID string) createdBacktest {
	return createdBacktest{
		name: name,
		exists: func(ctx context.Context) (bool, error) {
			_, err := backtests.IDByOwnerAndName(ctx, ownerID, name)
			if errors.Is(err, pgx.ErrNoRows) {
				return false, nil
			}
			return err == nil, err
		},
		remove: removeStoredBacktest(backtests, messageID),
	}
}

func removeStoredBacktest(backtests repository.Backtest, messageID string) func(context.Context) error {
	return func(ctx context.Context) error {
		id, err := backtests.IDByCreateMessage(ctx, messageID)
		if err != nil {
			return err
		}
		return backtests.DeleteByID(ctx, id)
	}
}
