package worker

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/internal/socket"
	finance_pb "github.com/lhjnilsson/foreverbull/pkg/finance/pb"
	worker_pb "github.com/lhjnilsson/foreverbull/pkg/service/pb"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/errgroup"
)

type Pool interface {
	Configure() *worker_pb.ExecutionConfiguration
	Process(ctx context.Context,
		timestamp time.Time, symbols []string, portfolio *finance_pb.Portfolio) ([]*finance_pb.Order, error)
	Close() error
}

func NewPool(ctx context.Context, algo *worker_pb.Algorithm) (Pool, error) {
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

	namespace := CreateNamespace(algo.Namespaces)

	p := &pool{Socket: poolSocket, NamespaceSocket: namespaceSocket, algo: algo, namespace: namespace}
	go p.startNamespaceListener()

	return p, nil
}

type pool struct {
	Socket          socket.Requester
	NamespaceSocket socket.Replier

	algo      *worker_pb.Algorithm
	namespace Namespace
}

func (p *pool) startNamespaceListener() {
	for {
		request := worker_pb.NamespaceRequest{}
		response := worker_pb.NamespaceResponse{}

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
		case worker_pb.NamespaceRequestType_GET:
			value := p.namespace.Get(request.Key)
			response.Value = value
		case worker_pb.NamespaceRequestType_SET:
			err := p.namespace.Set(request.Key, request.Value)
			if err != nil {
				error := err.Error()
				response.Error = &error
			}
		}

		err = sock.Reply(&response)
		if err != nil {
			log.Error().Err(err).Msg("error writing to namespace socket")
		}
	}
}

func (p *pool) orderedFunctions() <-chan *worker_pb.Algorithm_Function {
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

	return functionCh
}

func (p *pool) Configure() *worker_pb.ExecutionConfiguration {
	functions := make([]*worker_pb.ExecutionConfiguration_Function, 0)

	return &worker_pb.ExecutionConfiguration{
		BrokerPort:    int32(p.Socket.GetPort()),
		NamespacePort: int32(p.NamespaceSocket.GetPort()),
		DatabaseURL:   environment.GetPostgresURL(),
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

	var orders []*finance_pb.Order

	functions := p.orderedFunctions()
	for function := range functions {
		if function.ParallelExecution {
			group, _ := errgroup.WithContext(ctx)
			orderWriteMutex := sync.Mutex{}

			for _, symbol := range symbols {
				s := symbol

				group.Go(func() error {
					request := worker_pb.WorkerRequest{
						Task:      function.Name,
						Symbols:   []string{s},
						Portfolio: portfolio,
					}
					response := worker_pb.WorkerResponse{}

					err := p.Socket.Request(&request, &response)
					if err != nil {
						return fmt.Errorf("error processing request: %w", err)
					}

					orderWriteMutex.Lock()
					defer orderWriteMutex.Unlock()

					orders = append(orders, response.Orders...)

					return nil
				})
			}

			err := group.Wait()
			if err != nil {
				return nil, fmt.Errorf("error processing request: %w", err)
			}
		} else {
			request := worker_pb.WorkerRequest{
				Task:      function.Name,
				Symbols:   symbols,
				Portfolio: portfolio,
			}
			response := worker_pb.WorkerResponse{}

			err := p.Socket.Request(&request, &response)
			if err != nil {
				return nil, fmt.Errorf("error processing request: %w", err)
			}

			orders = append(orders, response.Orders...)
		}
	}

	return orders, nil
}

func (p *pool) Close() error {
	if p.Socket != nil {
		err := p.Socket.Close()
		if err != nil && !errors.Is(err, socket.ErrClosed) {
			return fmt.Errorf("error closing socket: %w", err)
		}
	}

	if p.NamespaceSocket != nil {
		err := p.NamespaceSocket.Close()
		if err != nil && !errors.Is(err, socket.ErrClosed) {
			return fmt.Errorf("error closing namespace socket: %w", err)
		}
	}

	return nil
}
