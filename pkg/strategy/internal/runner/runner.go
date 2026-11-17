package runner

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/lhjnilsson/foreverbull/internal/usercopy"
	"github.com/lhjnilsson/foreverbull/pkg/finance/supplier"
	commonpb "github.com/lhjnilsson/foreverbull/pkg/pb"
	financepb "github.com/lhjnilsson/foreverbull/pkg/pb/finance"
	pb "github.com/lhjnilsson/foreverbull/pkg/pb/strategy"
	"github.com/lhjnilsson/foreverbull/pkg/strategy/internal/repository"
	"github.com/rs/zerolog/log"
)

// ErrSkippedMissedClose is stored on a run when skip-missed is on and the
// process was down across a close.
var ErrSkippedMissedClose = errors.New("skipped_missed_close")

// ErrMissedClose is a session whose close passed while the deployment was
// down and skip-missed is off. The deployment is marked ERROR. The session
// is not stored as skipped, and last_session_date does not move.
var ErrMissedClose = errors.New("missed_session_close")

// missedCloseError is ErrMissedClose with the sentence stored on the deployment.
type missedCloseError struct {
	sentence string
}

func (e *missedCloseError) Error() string {
	if e == nil {
		return ErrMissedClose.Error()
	}
	return e.sentence
}

func (e *missedCloseError) Unwrap() error { return ErrMissedClose }

func missedClose(strategy string, dates []string) error {
	sentence, err := usercopy.MissedSessionClose(strategy, dates)
	if err != nil {
		return err
	}
	return &missedCloseError{sentence: sentence}
}

const defaultPollInterval = time.Minute

// Processor turns a session close into orders. Matches worker.Pool.Process.
// Tests inject a fake; production uses an optional worker.Pool when provided.
type Processor interface {
	Process(ctx context.Context, timestamp time.Time, symbols []string, portfolio *financepb.Portfolio) ([]*financepb.Order, error)
}

// InProcess runs one goroutine per running deployment.
// Each session resolves supplier.Trading from deployment.backend.
// PlaceOrder is called only when the deployment is paper with dry_run=false.
type InProcess struct {
	store     Store
	clock     Clock
	clients   supplier.Registry
	processor Processor
	poll      time.Duration

	rootCtx    context.Context
	rootCancel context.CancelFunc

	mu    sync.Mutex
	loops map[string]*loopHandle
	wg    sync.WaitGroup
}

// loopHandle stops one deployment loop; done closes once the loop has ended.
type loopHandle struct {
	cancel context.CancelFunc
	done   chan struct{}
}

func New(store Store, clock Clock, clients supplier.Registry, processor Processor) *InProcess {
	ctx, cancel := context.WithCancel(context.Background())
	return &InProcess{
		store:      store,
		clock:      clock,
		clients:    clients,
		processor:  processor,
		poll:       defaultPollInterval,
		rootCtx:    ctx,
		rootCancel: cancel,
		loops:      map[string]*loopHandle{},
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
	for _, handle := range r.loops {
		handle.cancel()
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
	handle := &loopHandle{cancel: cancel, done: make(chan struct{})}
	r.loops[d.Id] = handle
	r.wg.Add(1)
	go func(id string) {
		defer r.wg.Done()
		defer func() {
			r.mu.Lock()
			if r.loops[id] == handle {
				delete(r.loops, id)
			}
			r.mu.Unlock()
			close(handle.done)
		}()
		r.loop(loopCtx, id)
	}(d.Id)
	return nil
}

// Remove stops the loop for a deployment and waits until it has ended, so
// nothing the loop writes can land after Remove returns. The wait is bounded
// by ctx: when ctx ends first, Remove fails with ctx's cause and the loop
// stays listed until it ends, so a later Remove waits for it again.
// Idempotent: a deployment without a loop returns nil.
func (r *InProcess) Remove(ctx context.Context, deploymentID string) error {
	r.mu.Lock()
	handle, ok := r.loops[deploymentID]
	r.mu.Unlock()
	if !ok {
		return nil
	}
	handle.cancel()
	select {
	case <-handle.done:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("wait for deployment %s loop to stop: %w", deploymentID, context.Cause(ctx))
	}
}

func (r *InProcess) loop(ctx context.Context, deploymentID string) {
	for {
		if ctx.Err() != nil {
			return
		}

		d, err := r.store.GetDeployment(ctx, deploymentID)
		if err != nil {
			r.fail(ctx, deploymentID, fmt.Errorf("get deployment: %w", err))
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
			r.fail(ctx, deploymentID, fmt.Errorf("next close: %w", err))
			return
		}

		if r.clock.Now().After(session.Close) {
			// Skip-missed is off unless the deployment opted in. Every past
			// close is then named in one error and the loop ends. Nothing is
			// stored as skipped. With the flag, each close is recorded and
			// the loop walks on to the next one.
			if !d.GetSkipMissed() {
				missed, err := r.missedSoFar(ctx, d.CalendarCode, *session)
				if err != nil {
					r.fail(ctx, deploymentID, fmt.Errorf("missed closes: %w", err))
					return
				}
				r.fail(ctx, deploymentID, missedClose(d.GetStrategyName(), sessionDates(missed)))
				return
			}
			if err := r.SkipMissed(ctx, deploymentID, *session); err != nil {
				r.fail(ctx, deploymentID, fmt.Errorf("skip missed close: %w", err))
				return
			}
			continue
		}

		if err := r.clock.WaitUntil(ctx, session.Close); err != nil {
			return
		}
		if err := r.ProcessSession(ctx, deploymentID, *session); err != nil {
			r.fail(ctx, deploymentID, fmt.Errorf("process session: %w", err))
			return
		}
	}
}

// fail stores cause as the deployment's ERROR; the loop then ends. A loop
// stopped through ctx is not a failure: Remove waits for the loop to end
// before StopDeployment writes STOPPED, and at shutdown the row stays running
// so Start resumes it.
//
// The write is detached from ctx and bounded. The loop has no caller, so a
// status write that fails too is logged with both causes.
func (r *InProcess) fail(ctx context.Context, deploymentID string, cause error) {
	if ctx.Err() != nil {
		return
	}
	log.Error().Err(cause).Str("deployment", deploymentID).Msg("deployment runner failed")
	writeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), repository.StatusWriteTimeout)
	defer cancel()
	if err := r.store.UpdateStatus(writeCtx, deploymentID, pb.Deployment_ERROR, cause); err != nil {
		log.Error().Err(err).AnErr("cause", cause).Str("deployment", deploymentID).
			Msg("deployment runner failed and the deployment couldn't be marked as an error")
	}
}

// missedSoFar is first and every later close that is already past.
// The walk stops at the next close that has not happened, or when the
// calendar has no further session. One call of NextClose returns one close.
func (r *InProcess) missedSoFar(ctx context.Context, calendar string, first SessionClose) ([]SessionClose, error) {
	missed := []SessionClose{first}
	after := first.Date
	for {
		next, err := r.clock.NextClose(ctx, calendar, after)
		if errors.Is(err, ErrNoSession) {
			return missed, nil
		}
		if err != nil {
			return nil, err
		}
		if !r.clock.Now().After(next.Close) {
			return missed, nil
		}
		missed = append(missed, *next)
		after = next.Date
	}
}

func sessionDates(sessions []SessionClose) []string {
	dates := make([]string, len(sessions))
	for i, session := range sessions {
		dates[i] = commonpb.DateToDateString(session.Date)
	}
	return dates
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
// When dry_run is false it calls Trading.PlaceOrder for each order. Duplicate ticks no-op.
func (r *InProcess) ProcessSession(ctx context.Context, deploymentID string, session SessionClose) error {
	d, err := r.store.GetDeployment(ctx, deploymentID)
	if err != nil {
		return err
	}
	if dateEqual(d.LastSessionDate, session.Date) {
		return nil
	}

	var runErr error
	client, err := r.clientFor(d)
	if err != nil {
		runErr = err
	}
	var portfolio *financepb.Portfolio
	if runErr == nil && client != nil {
		portfolio, err = client.GetPortfolio(ctx)
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

	if runErr == nil && !d.DryRun {
		orders, runErr = r.placeOrders(ctx, client, orders)
	}

	_, err = r.store.CreateRun(ctx, deploymentID, session.Date, orders, false, runErr)
	if errors.Is(err, ErrDuplicateRun) {
		return r.store.SetLastSessionDate(ctx, deploymentID, session.Date)
	}
	if err != nil {
		return err
	}
	if err := r.store.SetLastSessionDate(ctx, deploymentID, session.Date); err != nil {
		return err
	}
	if runErr != nil && !d.DryRun {
		return runErr
	}
	return nil
}

func (r *InProcess) clientFor(d *pb.Deployment) (supplier.Trading, error) {
	if d == nil {
		return nil, errors.New("deployment is required")
	}
	name := strings.TrimSpace(d.Backend)
	if name == "" || r.clients == nil {
		return nil, nil
	}
	client, ok := r.clients.Get(name)
	if !ok || client == nil {
		return nil, fmt.Errorf("unknown trading profile %q", name)
	}
	return client, nil
}

func (r *InProcess) placeOrders(ctx context.Context, client supplier.Trading, orders []*financepb.Order) ([]*financepb.Order, error) {
	if client == nil {
		return orders, errors.New("PlaceOrder requires a Trading backend")
	}
	attempted := make([]*financepb.Order, 0, len(orders))
	for _, order := range orders {
		placed, err := client.PlaceOrder(ctx, order)
		if err != nil {
			attempted = append(attempted, order)
			return attempted, fmt.Errorf("place order: %w", err)
		}
		if placed != nil {
			attempted = append(attempted, placed)
			continue
		}
		attempted = append(attempted, order)
	}
	return attempted, nil
}
