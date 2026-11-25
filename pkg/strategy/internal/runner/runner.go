package runner

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/lhjnilsson/foreverbull/internal/usercopy"
	"github.com/lhjnilsson/foreverbull/pkg/finance"
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

// ErrDryRunPortfolio is a dry run whose trading profile could not be read.
// The deployment is marked ERROR. The stored text is the user sentence.
// Unwrap also carries the provider error, which is logged and not stored.
var ErrDryRunPortfolio = errors.New("dry_run_portfolio")

type dryRunPortfolioError struct {
	sentence string
	cause    error
}

func (e *dryRunPortfolioError) Error() string {
	if e == nil {
		return ErrDryRunPortfolio.Error()
	}
	return e.sentence
}

func (e *dryRunPortfolioError) Unwrap() []error {
	if e == nil || e.cause == nil {
		return []error{ErrDryRunPortfolio}
	}
	return []error{ErrDryRunPortfolio, e.cause}
}

func dryRunPortfolio(strategy, provider string, cause error) error {
	return &dryRunPortfolioError{
		sentence: usercopy.DryRunPortfolioUnreadable(strategy, provider),
		cause:    cause,
	}
}

const defaultPollInterval = time.Minute

// Processor turns a session close into orders. Matches worker.Pool.Process.
// Tests inject a fake; production uses an optional worker.Pool when provided.
type Processor interface {
	Process(ctx context.Context, timestamp time.Time, symbols []string, portfolio *financepb.Portfolio) ([]*financepb.Order, error)
}

// InProcess runs one goroutine per running deployment.
// A session with dry_run false resolves supplier.Trading from deployment.backend.
// PlaceOrder is called only when the deployment is paper with dry_run=false.
// A dry run reads that portfolio when a profile is set, and uses an empty
// portfolio when it is not. It never calls PlaceOrder.
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
// A deployment that is not admitted is already ERROR. Start logs that and
// keeps going, so one refusal does not stop the broker. Start returns an
// error only when the running list cannot be read, or when a deployment's
// ERROR status could not be written and the row is still RUNNING.
func (r *InProcess) Start(ctx context.Context) error {
	running, err := r.store.ListRunning(ctx)
	if err != nil {
		return fmt.Errorf("list running deployments: %w", err)
	}
	for _, d := range running {
		err := r.Add(ctx, d)
		if err == nil {
			continue
		}
		var admitted *AdmissionError
		if errors.As(err, &admitted) {
			stored, readErr := r.store.GetDeployment(ctx, d.GetId())
			if readErr == nil && stored.GetStatus() == pb.Deployment_ERROR {
				cause := admitted.Cause
				if cause == nil {
					cause = err
				}
				log.Error().Err(cause).Str("deployment", d.GetId()).Str("cause", admitted.Name).
					Msg("deployment was not admitted")
				continue
			}
		}
		// The row is still RUNNING. Stop every loop this start already
		// opened, then return the error so the broker does not keep trading
		// on a boot that failed.
		if stopErr := r.Stop(ctx); stopErr != nil {
			return errors.Join(err, fmt.Errorf("stop deployments started before the status write failed: %w", stopErr))
		}
		return err
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

// AdmissionError is a deployment that must not start. Error is the sentence
// stored on the row and returned to deploy. Cause is the underlying failure
// when there is one.
type AdmissionError struct {
	Sentence string
	Cause    error
	// Name is the log cause: listing_refused, listing_unknown, listing_lookup,
	// profile, or store.
	Name string
}

func (e *AdmissionError) Error() string {
	if e == nil || e.Sentence == "" {
		return "deployment was not admitted"
	}
	return e.Sentence
}

func (e *AdmissionError) Unwrap() error { return e.Cause }

// Add starts a loop for a running deployment. Idempotent per deployment id.
// Each deployment is checked once, on the add, against its symbols and the
// profile's price provider. A deployment has no benchmark of its own.
// A pass stores the decision so a later add does not ask again. Anything
// that is not a pass marks the deployment ERROR and returns that sentence.
func (r *InProcess) Add(ctx context.Context, d *pb.Deployment) error {
	if d == nil || d.Id == "" {
		return fmt.Errorf("deployment is required")
	}

	r.mu.Lock()
	if _, ok := r.loops[d.Id]; ok {
		r.mu.Unlock()
		return nil
	}
	r.mu.Unlock()

	if err := r.admit(ctx, d); err != nil {
		if updateErr := r.store.UpdateStatus(ctx, d.GetId(), pb.Deployment_ERROR, err); updateErr != nil {
			log.Error().Err(updateErr).Str("deployment", d.GetId()).Str("cause", "store").
				Msg("deployment could not be marked failed")
			return &AdmissionError{Sentence: err.Error(), Cause: updateErr, Name: "store"}
		}
		var admitted *AdmissionError
		name := "listing_lookup"
		if errors.As(err, &admitted) && admitted.Name != "" {
			name = admitted.Name
		}
		log.Error().Err(err).Str("deployment", d.GetId()).Str("cause", name).
			Msg("deployment was not admitted")
		return err
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

// admit reports whether the deployment may start. A stored pass, and a
// deployment with no symbols, may start. Every other result is an
// AdmissionError whose sentence is what the row stores.
func (r *InProcess) admit(ctx context.Context, d *pb.Deployment) error {
	checked, err := r.store.MarketsChecked(ctx, d.GetId())
	if err != nil {
		return &AdmissionError{Sentence: usercopy.DeploymentListingNotStored(d.GetStrategyName()), Cause: err, Name: "store"}
	}
	if checked {
		return nil
	}
	if !hasSymbol(d.GetSymbols()) {
		if err := r.store.MarkMarketsChecked(ctx, d.GetId()); err != nil {
			return &AdmissionError{Sentence: usercopy.DeploymentListingNotStored(d.GetStrategyName()), Cause: err, Name: "store"}
		}
		return nil
	}
	provider, err := finance.ListingProvider(d.GetBackend())
	if err != nil {
		return &AdmissionError{
			Sentence: usercopy.DeploymentProfileUnchecked(d.GetStrategyName(), strings.TrimSpace(d.GetBackend()), !errors.Is(err, finance.ErrProfileNotFound)),
			Cause:    err,
			Name:     "profile",
		}
	}
	profile := strings.TrimSpace(d.GetBackend())
	listed, _, err := finance.RefuseListed(ctx, provider, profile, d.GetSymbols(), "")
	if err != nil {
		names, lookupProvider := admissionLookups(err)
		if len(names) > 0 {
			return &AdmissionError{Sentence: usercopy.LookupDeploy(d.GetStrategyName(), lookupProvider, names), Cause: err, Name: "listing_lookup"}
		}
		return &AdmissionError{Sentence: usercopy.DeploymentListingUnexpected(d.GetStrategyName()), Cause: err, Name: "listing_lookup"}
	}
	if len(listed) > 0 {
		return &AdmissionError{Sentence: usercopy.GlobalMarketsDeploy(d.GetStrategyName(), listed), Name: "listing_refused"}
	}
	if !finance.ListingsAnswered(provider, d.GetSymbols()) {
		return &AdmissionError{
			Sentence: usercopy.DeploymentSymbolsUnknown(d.GetStrategyName(), provider, finance.Unanswered(provider, d.GetSymbols())),
			Name:     "listing_unknown",
		}
	}
	if err := r.store.MarkMarketsChecked(ctx, d.GetId()); err != nil {
		return &AdmissionError{Sentence: usercopy.DeploymentListingNotStored(d.GetStrategyName()), Cause: err, Name: "store"}
	}
	return nil
}

func admissionLookups(err error) (names []string, provider string) {
	var many *finance.QuoteFailures
	if errors.As(err, &many) && many != nil {
		for _, item := range many.Items {
			if item == nil {
				continue
			}
			names = append(names, item.Symbol)
			if provider == "" {
				provider = item.Provider
			}
		}
		return names, provider
	}
	var one *finance.QuoteFailure
	if errors.As(err, &one) && one != nil {
		return []string{one.Symbol}, one.Provider
	}
	return nil, ""
}

func hasSymbol(symbols []string) bool {
	for _, symbol := range symbols {
		if strings.TrimSpace(symbol) != "" {
			return true
		}
	}
	return false
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

// ProcessSession runs Process and persists strategy_deployment_run.
// A dry run with a trading profile reads GetPortfolio and does not call
// PlaceOrder. A dry run with no profile uses an empty portfolio. When
// dry_run is false it calls GetPortfolio and then Trading.PlaceOrder.
// Duplicate ticks no-op.
func (r *InProcess) ProcessSession(ctx context.Context, deploymentID string, session SessionClose) error {
	d, err := r.store.GetDeployment(ctx, deploymentID)
	if err != nil {
		return err
	}
	if dateEqual(d.LastSessionDate, session.Date) {
		return nil
	}

	var runErr error
	var client supplier.Trading
	var portfolio *financepb.Portfolio
	var dryNote string
	profile := strings.TrimSpace(d.GetBackend())
	if d.DryRun && profile == "" {
		portfolio = &financepb.Portfolio{}
		dryNote = usercopy.DryRunEmptyPortfolio
	} else if d.DryRun {
		client, err = r.tradingClient(d)
		if err == nil {
			portfolio, err = client.GetPortfolio(ctx)
		}
		if err != nil {
			log.Error().Err(err).Str("deployment", deploymentID).Str("backend", profile).
				Msg("dry run could not read the portfolio")
			failed := dryRunPortfolio(d.GetStrategyName(), profile, err)
			r.fail(ctx, deploymentID, failed)
			return failed
		}
		dryNote = usercopy.DryRunRecorded(profile)
	} else {
		client, err = r.clientFor(d)
		if err != nil {
			runErr = err
		}
		if runErr == nil && client != nil {
			portfolio, err = client.GetPortfolio(ctx)
			if err != nil {
				runErr = fmt.Errorf("get portfolio: %w", err)
			}
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
	if dryNote != "" && runErr == nil {
		log.Info().Str("deployment", deploymentID).Str("backend", profile).Msg(dryNote)
	}
	return nil
}

// tradingClient resolves a deployment that names a trading profile.
// An empty registry or an unknown name is an error. A deployment with no
// profile does not call this.
func (r *InProcess) tradingClient(d *pb.Deployment) (supplier.Trading, error) {
	name := strings.TrimSpace(d.GetBackend())
	if name == "" || r.clients == nil {
		return nil, fmt.Errorf("unknown trading profile %q", name)
	}
	client, ok := r.clients.Get(name)
	if !ok || client == nil {
		return nil, fmt.Errorf("unknown trading profile %q", name)
	}
	return client, nil
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
