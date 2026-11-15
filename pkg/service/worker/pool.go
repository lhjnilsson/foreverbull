package worker

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/lhjnilsson/foreverbull/internal/socket"
	"github.com/lhjnilsson/foreverbull/internal/usercopy"
	finance_pb "github.com/lhjnilsson/foreverbull/pkg/pb/finance"
	worker_pb "github.com/lhjnilsson/foreverbull/pkg/pb/service"
	servicePb "github.com/lhjnilsson/foreverbull/pkg/pb/service/service"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/errgroup"
)

type Pool interface {
	Configure() *worker_pb.ExecutionConfiguration
	Process(ctx context.Context,
		timestamp time.Time, symbols []string, portfolio *finance_pb.Portfolio) ([]*finance_pb.Order, error)
	Close() error
}

// BarSource loads stored bars for one worker step. The broker calls it;
// the worker never opens Postgres.
type BarSource interface {
	List(ctx context.Context, provider string, symbols []string, asOf time.Time, limit int) ([]*finance_pb.OHLC, error)
}

// Option configures a pool.
type Option func(*pool)

// WithBars attaches stored bars to each WorkerRequest. provider is the
// price source the caller already resolved. The pool passes that name
// through and does not choose one.
func WithBars(source BarSource, provider string) Option {
	return func(p *pool) {
		p.bars = source
		p.provider = provider
	}
}

// SessionCalendar is the XNYS calendar a trading step reads. LastSession
// is the latest session whose close, plus the 30-minute finality margin,
// is at or before asOf. The time is midnight UTC on that session's date.
// A calendar with no such session returns ErrNoCompletedSession. The pool
// does not invent a weekday.
type SessionCalendar interface {
	LastSession(ctx context.Context, asOf time.Time) (time.Time, error)
}

// CalendarSession is one row of that calendar.
type CalendarSession struct {
	// Date is the session date. The UTC calendar day is the day named.
	Date time.Time
	// Close is the session close, an absolute instant.
	Close time.Time
}

// sessionFinality is the 30 minutes after the close before a session
// counts as finished, the same margin a price fetch stops at.
const sessionFinality = 30 * time.Minute

// ErrNoCompletedSession is a trading clock with no finished XNYS session
// at or before asOf. The pool does not substitute a day.
var ErrNoCompletedSession = errors.New("no completed session")

// ErrCalendarNotConfigured is a trading step with no XNYS calendar.
var ErrCalendarNotConfigured = errors.New("xnys calendar is not configured")

// ErrCalendarRead is a trading step whose XNYS calendar query failed.
// The user sentence does not include the database error. Unwrap keeps it
// for errors.Is and errors.As.
var ErrCalendarRead = errors.New("xnys calendar read failed")

// ErrMissingAsOfBar is a trading run whose newest bar is older than the
// as-of session. The error that wraps it names the provider, every such
// symbol, and that date. Trading does not use an older bar.
var ErrMissingAsOfBar = errors.New("no bar on the as-of session")

// WithSessionBarRequired fails a trading step when a symbol's newest bar
// is older than the as-of session. calendar supplies that session. Trading
// sets it. A backtest does not: that path still hands over the bars it
// loaded, including an older day. A nil calendar is not a guess: the step
// fails with ErrCalendarNotConfigured.
func WithSessionBarRequired(calendar SessionCalendar) Option {
	return func(p *pool) {
		p.requireSessionBar = true
		p.calendar = calendar
	}
}

const (
	workerBarChunk = 500
	workerBarLimit = 252
)

func NewPool(ctx context.Context, algo *worker_pb.Algorithm, opts ...Option) (Pool, error) {
	poolSocket, err := socket.NewRequester("0.0.0.0", 0, false)
	if err != nil {
		return nil, fmt.Errorf("error creating requester: %w", err)
	}

	namespaceSocket, err := socket.NewReplier("0.0.0.0", 0, false)
	if err != nil {
		return nil, fmt.Errorf("error creating replier: %w", err)
	}

	if algo == nil {
		return nil, errors.New("algorithm is not set")
	}

	if len(algo.Functions) == 0 {
		log.Warn().Any("algorithm", algo).Msg("no functions in algorithm")
	}

	namespace := CreateNamespace(algo.Namespaces)

	p := &pool{Socket: poolSocket, NamespaceSocket: namespaceSocket, algo: algo, namespace: namespace}
	for _, opt := range opts {
		opt(p)
	}
	go p.startNamespaceListener()

	return p, nil
}

type pool struct {
	Socket          socket.Requester
	NamespaceSocket socket.Replier

	algo      *worker_pb.Algorithm
	namespace Namespace

	bars              BarSource
	provider          string
	requireSessionBar bool
	calendar          SessionCalendar
}

func (p *pool) startNamespaceListener() {
	for {
		request := servicePb.NamespaceRequest{}
		response := servicePb.NamespaceResponse{}

		sock, err := p.NamespaceSocket.Receive(&request)
		if err != nil {
			if errors.Is(err, socket.ErrClosed) {
				log.Info().Msg("namespace socket closed")
				return
			}

			log.Error().Err(err).Msg("error reading from namespace socket")

			continue
		}

		switch request.Type {
		case servicePb.NamespaceRequestType_GET:
			value := p.namespace.Get(request.Key)
			response.Value = value
		case servicePb.NamespaceRequestType_SET:
			err := p.namespace.Set(request.Key, request.Value)
			if err != nil {
				setError := err.Error()
				response.Error = &setError
			}
		}

		err = sock.Reply(&response)
		if err != nil {
			log.Error().Err(err).Msg("error writing to namespace socket")
		}
	}
}

func (p *pool) orderedFunctions() (<-chan *worker_pb.Algorithm_Function, error) {
	if p.algo == nil {
		return nil, errors.New("algorithm not set")
	}
	if len(p.algo.Functions) == 0 {
		return nil, errors.New("no functions in algorithm")
	}

	functionCh := make(chan *worker_pb.Algorithm_Function)
	go func() {
		for _, function := range p.algo.Functions {
			if function.RunFirst && !function.RunLast {
				functionCh <- function
			}
		}

		for _, function := range p.algo.Functions {
			if !function.RunFirst && !function.RunLast {
				functionCh <- function
			}
		}

		for _, function := range p.algo.Functions {
			if !function.RunFirst && function.RunLast {
				functionCh <- function
			}
		}

		close(functionCh)
	}()

	return functionCh, nil
}

func (p *pool) Configure() *worker_pb.ExecutionConfiguration {
	functions := make([]*worker_pb.ExecutionConfiguration_Function, 0)

	return &worker_pb.ExecutionConfiguration{
		BrokerPort:    int32(p.Socket.GetPort()),
		NamespacePort: int32(p.NamespaceSocket.GetPort()),
		Functions:     functions,
	}
}

func (p *pool) Process(ctx context.Context, timestamp time.Time, symbols []string,
	portfolio *finance_pb.Portfolio,
) ([]*finance_pb.Order, error) {
	if p.algo == nil {
		return nil, errors.New("algorithm not set")
	}

	p.namespace.Flush()

	asOf := timestamp
	if portfolio != nil && portfolio.GetTimestamp() != nil {
		asOf = portfolio.GetTimestamp().AsTime()
	}
	loaded, err := p.lookupBars(ctx, symbols, asOf)
	if err != nil {
		return nil, err
	}

	var orders []*finance_pb.Order

	functions, err := p.orderedFunctions()
	if err != nil {
		return nil, fmt.Errorf("error getting ordered functions: %w", err)
	}
	for function := range functions {
		if function.ParallelExecution {
			group, _ := errgroup.WithContext(ctx)
			orderWriteMutex := sync.Mutex{}

			for _, symbol := range symbols {
				s := symbol

				group.Go(func() error {
					request := servicePb.WorkerRequest{
						Task:      function.Name,
						Symbols:   []string{s},
						Portfolio: portfolio,
						Bars:      barsForSymbol(loaded, s),
					}
					response := servicePb.WorkerResponse{}

					err := p.Socket.Request(&request, &response)
					if err != nil {
						return fmt.Errorf("error processing request: %w", err)
					}
					if err := workerResponseError(&response); err != nil {
						return err
					}

					orderWriteMutex.Lock()
					defer orderWriteMutex.Unlock()

					orders = append(orders, response.Orders...)

					return nil
				})
			}

			err := group.Wait()
			if err != nil {
				return nil, err
			}
		} else {
			request := servicePb.WorkerRequest{
				Task:      function.Name,
				Symbols:   symbols,
				Portfolio: portfolio,
				Bars:      loaded,
			}
			response := servicePb.WorkerResponse{}

			err := p.Socket.Request(&request, &response)
			if err != nil {
				return nil, fmt.Errorf("error processing request: %w", err)
			}
			if err := workerResponseError(&response); err != nil {
				return nil, err
			}

			orders = append(orders, response.Orders...)
		}
	}

	return orders, nil
}

// workerResponseError is the worker's own failure. The text is already the
// sentence a caller should show. A market-data permission failure is the
// install-bug sentence, with no SQL. The CLI adds the backtest name.
func workerResponseError(response *servicePb.WorkerResponse) error {
	if response == nil {
		return nil
	}
	message := strings.TrimSpace(response.GetError())
	if message == "" {
		return nil
	}
	return errors.New(message)
}

func (p *pool) lookupBars(ctx context.Context, symbols []string, asOf time.Time) ([]*finance_pb.OHLC, error) {
	if p.bars == nil || len(symbols) == 0 {
		return nil, nil
	}
	loaded := make([]*finance_pb.OHLC, 0)
	for start := 0; start < len(symbols); start += workerBarChunk {
		end := start + workerBarChunk
		if end > len(symbols) {
			end = len(symbols)
		}
		chunk, err := p.bars.List(ctx, p.provider, symbols[start:end], asOf, workerBarLimit)
		if err != nil {
			return nil, fmt.Errorf("load bars: %w", err)
		}
		loaded = append(loaded, chunk...)
	}
	missing := missingBarSymbols(symbols, loaded)
	if len(missing) > 0 {
		return nil, missingBarsError(missing, asOf)
	}
	if p.requireSessionBar {
		// The portfolio clock is often the next morning, a Saturday, or
		// the day after a holiday. The session is the calendar's, not a
		// day counted back from the clock. A newest bar older than that
		// session is refused. A bar on the session, or a later one List
		// already capped at asOf, is the history the strategy gets.
		session, err := p.asOfSession(ctx, asOf)
		if err != nil {
			return nil, err
		}
		stale := symbolsWithoutSessionBar(symbols, loaded, session)
		if len(stale) > 0 {
			return nil, missingAsOfBarError(p.provider, stale, session)
		}
	}
	return loaded, nil
}

// missingBarSymbols are the requested symbols with no loaded bar, in request
// order, once each. An empty request is not a missing symbol.
func missingBarSymbols(requested []string, bars []*finance_pb.OHLC) []string {
	present := make(map[string]struct{}, len(bars))
	for _, bar := range bars {
		present[bar.GetSymbol()] = struct{}{}
	}
	missing := make([]string, 0)
	seen := make(map[string]struct{}, len(requested))
	for _, symbol := range requested {
		if _, ok := seen[symbol]; ok {
			continue
		}
		seen[symbol] = struct{}{}
		if _, ok := present[symbol]; ok {
			continue
		}
		missing = append(missing, symbol)
	}
	return missing
}

// missingBarsError names every requested symbol the load did not deliver.
// An empty bar list is a failed delivery, not a flat day: a strategy that
// sees no rows can sell a position it still holds. The date is the UTC
// calendar day, the same day the client names when it builds stock_data.
func missingBarsError(symbols []string, asOf time.Time) error {
	return fmt.Errorf("no bars for %s on %s", joinSymbols(symbols), asOf.UTC().Format(time.DateOnly))
}

// symbolsWithoutSessionBar are the requested symbols whose newest bar is
// older than the session, in request order, once each. session is the
// as-of session at midnight UTC. A symbol with no timestamped bar is
// included. A bar on the session date, or a later date, is not.
func symbolsWithoutSessionBar(requested []string, bars []*finance_pb.OHLC, session time.Time) []string {
	missing := make([]string, 0)
	seen := make(map[string]struct{}, len(requested))
	for _, symbol := range requested {
		if _, ok := seen[symbol]; ok {
			continue
		}
		seen[symbol] = struct{}{}
		newest, ok := newestBar(bars, symbol)
		if !ok || barIsOlderThanSession(newest, session) {
			missing = append(missing, symbol)
		}
	}
	return missing
}

// newestBar is the latest timestamp stored for symbol. A bar with no
// timestamp is not a session.
func newestBar(bars []*finance_pb.OHLC, symbol string) (time.Time, bool) {
	var newest time.Time
	found := false
	for _, bar := range bars {
		if bar == nil || bar.GetSymbol() != symbol || bar.GetTimestamp() == nil {
			continue
		}
		when := bar.GetTimestamp().AsTime()
		if !found || when.After(newest) {
			newest = when
			found = true
		}
	}
	return newest, found
}

// barIsOlderThanSession is true when the bar's UTC date is before the
// session date. A bar on that date or a later one is not an older price.
func barIsOlderThanSession(bar, session time.Time) bool {
	by, bm, bd := bar.UTC().Date()
	sy, sm, sd := session.UTC().Date()
	barDay := time.Date(by, bm, bd, 0, 0, 0, 0, time.UTC)
	sessionDay := time.Date(sy, sm, sd, 0, 0, 0, 0, time.UTC)
	return barDay.Before(sessionDay)
}

func (p *pool) asOfSession(ctx context.Context, asOf time.Time) (time.Time, error) {
	if p.calendar == nil {
		return time.Time{}, CalendarNotConfigured()
	}
	session, err := p.calendar.LastSession(ctx, asOf)
	if err != nil {
		if errors.Is(err, ErrNoCompletedSession) {
			return time.Time{}, missingCompletedSession(asOf)
		}
		// A calendar failure already has its user sentence. Prefixing it
		// would put the database text, or a second wrapper, on Run.
		return time.Time{}, err
	}
	return session, nil
}

// LastFinishedSession is the latest session whose close plus the 30-minute
// finality margin is at or before asOf. sessions are calendar rows the
// caller loaded. The result is midnight UTC on that session's date. No
// such row is ErrNoCompletedSession. The function does not know which
// weekdays or holidays the exchange closes.
func LastFinishedSession(sessions []CalendarSession, asOf time.Time) (time.Time, error) {
	var best time.Time
	found := false
	for _, session := range sessions {
		if session.Close.Add(sessionFinality).After(asOf) {
			continue
		}
		day := utcDay(session.Date)
		if !found || day.After(best) {
			best = day
			found = true
		}
	}
	if !found {
		return time.Time{}, ErrNoCompletedSession
	}
	return best, nil
}

func utcDay(t time.Time) time.Time {
	year, month, day := t.UTC().Date()
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}

// noCompletedSessionError names the clock reading. It wraps
// ErrNoCompletedSession. The pool does not trade on a guessed day.
type noCompletedSessionError struct {
	at string
}

func missingCompletedSession(asOf time.Time) error {
	return &noCompletedSessionError{at: asOf.UTC().Format(time.RFC3339)}
}

func (e *noCompletedSessionError) Error() string {
	return "trading: foreverbull's XNYS calendar has no finished trading session at or before " + e.at + ", so it won't trade. " + usercopy.ReportIt
}

func (e *noCompletedSessionError) Unwrap() error {
	return ErrNoCompletedSession
}

// asOfBarError names the provider and every symbol whose newest bar is
// older than the as-of session. The sentence is the whole message a
// caller shows. It wraps ErrMissingAsOfBar. fbull trading run is the
// path that sets this check, so the ending tells the user to run it again.
type asOfBarError struct {
	provider string
	symbols  []string
	day      string
}

func missingAsOfBarError(provider string, symbols []string, session time.Time) error {
	return &asOfBarError{
		provider: provider,
		symbols:  symbols,
		day:      session.UTC().Format(time.DateOnly),
	}
}

func (e *asOfBarError) Error() string {
	kind := "price"
	if len(e.symbols) != 1 {
		kind = "prices"
	}
	return fmt.Sprintf(
		"trading: %s has no %s for %s on %s, the latest finished XNYS session, so foreverbull won't trade on older prices. Run `fbull trading run` again once %s has that day's prices.",
		e.provider, kind, joinSymbols(e.symbols), e.day, e.provider,
	)
}

func (e *asOfBarError) Unwrap() error {
	return ErrMissingAsOfBar
}

// CalendarNotConfigured is the refusal when trading has no XNYS calendar.
// It wraps ErrCalendarNotConfigured.
func CalendarNotConfigured() error {
	return calendarNotConfiguredError{}
}

type calendarNotConfiguredError struct{}

func (calendarNotConfiguredError) Error() string {
	return "trading: this broker wasn't set up with the XNYS trading calendar, so it won't trade. " + usercopy.ReportIt
}

func (calendarNotConfiguredError) Unwrap() error {
	return ErrCalendarNotConfigured
}

// CalendarReadFailed is the refusal when the XNYS calendar query fails.
// The sentence does not include cause. Unwrap keeps ErrCalendarRead and
// cause for errors.Is and errors.As.
func CalendarReadFailed(cause error) error {
	return &calendarReadError{cause: cause}
}

type calendarReadError struct {
	cause error
}

func (e *calendarReadError) Error() string {
	// ReportIt starts with a capital Please. This ending is lowercase
	// after "If it repeats,", so it uses the issues URL on its own.
	return "trading: foreverbull couldn't read its XNYS trading calendar, so it won't trade. " +
		usercopy.RetryStep("fbull trading run") +
		" If it repeats, please report it at " + usercopy.IssuesURL + " with this message."
}

func (e *calendarReadError) Unwrap() []error {
	if e == nil || e.cause == nil {
		return []error{ErrCalendarRead}
	}
	return []error{ErrCalendarRead, e.cause}
}

// joinSymbols writes A, A and B, or A, B and C.
func joinSymbols(symbols []string) string {
	if len(symbols) < 2 {
		return strings.Join(symbols, "")
	}
	return strings.Join(symbols[:len(symbols)-1], ", ") + " and " + symbols[len(symbols)-1]
}

func barsForSymbol(bars []*finance_pb.OHLC, symbol string) []*finance_pb.OHLC {
	matched := make([]*finance_pb.OHLC, 0)
	for _, bar := range bars {
		if bar.GetSymbol() == symbol {
			matched = append(matched, bar)
		}
	}
	return matched
}

// Close closes both sockets, even when the first fails, and returns every
// failure. A socket that was already closed is not one.
func (p *pool) Close() error {
	var errs []error
	if p.Socket != nil {
		err := p.Socket.Close()
		if err != nil && !errors.Is(err, socket.ErrClosed) {
			errs = append(errs, fmt.Errorf("error closing socket: %w", err))
		}
	}

	if p.NamespaceSocket != nil {
		err := p.NamespaceSocket.Close()
		if err != nil && !errors.Is(err, socket.ErrClosed) {
			errs = append(errs, fmt.Errorf("error closing namespace socket: %w", err))
		}
	}

	return errors.Join(errs...)
}
