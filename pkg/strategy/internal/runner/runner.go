package runner

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/lhjnilsson/foreverbull/pkg/finance/supplier"
	commonpb "github.com/lhjnilsson/foreverbull/pkg/pb"
	financepb "github.com/lhjnilsson/foreverbull/pkg/pb/finance"
	pb "github.com/lhjnilsson/foreverbull/pkg/pb/strategy"
	"github.com/rs/zerolog/log"
)

// ErrSkippedMissedClose is stored on a run when the process was down across a close.
var ErrSkippedMissedClose = errors.New("skipped_missed_close")

const defaultPollInterval = time.Minute

// Processor turns a session close into orders. Matches worker.Pool.Process.
// Tests inject a fake; production may be nil (log-only dry-run).
type Processor interface {
	Process(ctx context.Context, timestamp time.Time, symbols []string, portfolio *financepb.Portfolio) ([]*financepb.Order, error)
}

// InProcess runs one goroutine per running deployment. It never calls PlaceOrder.
type InProcess struct {
	store     Store
	clock     Clock
	trading   supplier.Trading
	processor Processor
	poll      time.Duration

	rootCtx    context.Context
	rootCancel context.CancelFunc

	mu    sync.Mutex
	loops map[string]context.CancelFunc
	wg    sync.WaitGroup
}

func New(store Store, clock Clock, trading supplier.Trading, processor Processor) *InProcess {
	ctx, cancel := context.WithCancel(context.Background())
	return &InProcess{
		store:      store,
		clock:      clock,
		trading:    trading,
		processor:  processor,
		poll:       defaultPollInterval,
		rootCtx:    ctx,
		rootCancel: cancel,
		loops:      map[string]context.CancelFunc{},
	}
}

func (r *InProcess) WithPollInterval(d time.Duration) *InProcess {
	r.poll = d
	return r
}

// Start recovers deployments with status=running from last_session_date.
func (r *InProcess) Start(ctx context.Context) error {
	running, err := r.store.ListRunning(ctx)
	if err != nil {
		return fmt.Errorf("list running deployments: %w", err)
	}
	for _, d := range running {
		if err := r.Add(ctx, d); err != nil {
			return err
		}
	}
	return nil
}

func (r *InProcess) Stop(ctx context.Context) error {
	r.rootCancel()

	r.mu.Lock()
	for id, cancel := range r.loops {
		cancel()
		delete(r.loops, id)
	}
	r.mu.Unlock()

	done := make(chan struct{})
	go func() {
		r.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Add starts a loop for a running deployment. Idempotent per deployment id.
func (r *InProcess) Add(_ context.Context, d *pb.Deployment) error {
	if d == nil || d.Id == "" {
		return fmt.Errorf("deployment is required")
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.loops[d.Id]; ok {
		return nil
	}

	loopCtx, cancel := context.WithCancel(r.rootCtx)
	r.loops[d.Id] = cancel
	r.wg.Add(1)
	go func(id string) {
		defer r.wg.Done()
		r.loop(loopCtx, id)
	}(d.Id)
	return nil
}

// Remove stops the loop for a deployment. Idempotent.
func (r *InProcess) Remove(_ context.Context, deploymentID string) error {
	r.mu.Lock()
	cancel, ok := r.loops[deploymentID]
	if ok {
		delete(r.loops, deploymentID)
	}
	r.mu.Unlock()
	if ok {
		cancel()
	}
	return nil
}

func (r *InProcess) loop(ctx context.Context, deploymentID string) {
	for {
		if ctx.Err() != nil {
			return
		}

		d, err := r.store.GetDeployment(ctx, deploymentID)
		if err != nil {
			log.Error().Err(err).Str("deployment", deploymentID).Msg("runner get deployment")
			return
		}
		if d.Status != pb.Deployment_RUNNING {
			return
		}

		session, err := r.clock.NextClose(ctx, d.CalendarCode, d.LastSessionDate)
		if errors.Is(err, ErrNoSession) {
			select {
			case <-ctx.Done():
				return
			case <-time.After(r.poll):
			}
			continue
		}
		if err != nil {
			log.Error().Err(err).Str("deployment", deploymentID).Msg("runner next close")
			_ = r.store.UpdateStatus(ctx, deploymentID, pb.Deployment_ERROR, err)
			return
		}

		if r.clock.Now().After(session.Close) {
			if err := r.SkipMissed(ctx, deploymentID, *session); err != nil {
				log.Error().Err(err).Str("deployment", deploymentID).Msg("runner skip missed close")
				_ = r.store.UpdateStatus(ctx, deploymentID, pb.Deployment_ERROR, err)
				return
			}
			continue
		}

		if err := r.clock.WaitUntil(ctx, session.Close); err != nil {
			return
		}
		if err := r.ProcessSession(ctx, deploymentID, *session); err != nil {
			log.Error().Err(err).Str("deployment", deploymentID).Msg("runner process session")
			_ = r.store.UpdateStatus(ctx, deploymentID, pb.Deployment_ERROR, err)
			return
		}
	}
}

// SkipMissed records a missed close and advances last_session_date. No Process, no PlaceOrder.
func (r *InProcess) SkipMissed(ctx context.Context, deploymentID string, session SessionClose) error {
	d, err := r.store.GetDeployment(ctx, deploymentID)
	if err != nil {
		return err
	}
	if dateEqual(d.LastSessionDate, session.Date) {
		return nil
	}

	_, err = r.store.CreateRun(ctx, deploymentID, session.Date, nil, true, ErrSkippedMissedClose)
	if errors.Is(err, ErrDuplicateRun) {
		return r.store.SetLastSessionDate(ctx, deploymentID, session.Date)
	}
	if err != nil {
		return err
	}
	log.Info().
		Str("deployment", deploymentID).
		Str("session_date", commonpb.DateToDateString(session.Date)).
		Msg("skipped_missed_close")
	return r.store.SetLastSessionDate(ctx, deploymentID, session.Date)
}

// ProcessSession runs GetPortfolio + Process and persists strategy_deployment_run.
// It never calls PlaceOrder. Duplicate ticks (same last_session_date / unique run) no-op.
func (r *InProcess) ProcessSession(ctx context.Context, deploymentID string, session SessionClose) error {
	d, err := r.store.GetDeployment(ctx, deploymentID)
	if err != nil {
		return err
	}
	if dateEqual(d.LastSessionDate, session.Date) {
		return nil
	}

	var runErr error
	var portfolio *financepb.Portfolio
	if r.trading != nil {
		portfolio, err = r.trading.GetPortfolio()
		if err != nil {
			runErr = fmt.Errorf("get portfolio: %w", err)
		}
	}

	var orders []*financepb.Order
	if runErr == nil && r.processor != nil {
		orders, err = r.processor.Process(ctx, session.Close, d.Symbols, portfolio)
		if err != nil {
			runErr = fmt.Errorf("process: %w", err)
			orders = nil
		}
	}

	// PoC: never PlaceOrder. dry_run paper logs would-be orders only.
	_, err = r.store.CreateRun(ctx, deploymentID, session.Date, orders, false, runErr)
	if errors.Is(err, ErrDuplicateRun) {
		return r.store.SetLastSessionDate(ctx, deploymentID, session.Date)
	}
	if err != nil {
		return err
	}
	return r.store.SetLastSessionDate(ctx, deploymentID, session.Date)
}
