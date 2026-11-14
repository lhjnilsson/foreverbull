package worker

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/lhjnilsson/foreverbull/internal/socket"
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

	bars     BarSource
	provider string
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
