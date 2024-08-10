package worker

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/lhjnilsson/foreverbull/internal/pb"
	finance_pb "github.com/lhjnilsson/foreverbull/internal/pb/finance"
	service_pb "github.com/lhjnilsson/foreverbull/internal/pb/service"
	"github.com/lhjnilsson/foreverbull/internal/socket"
	finance "github.com/lhjnilsson/foreverbull/pkg/finance/entity"
	"github.com/lhjnilsson/foreverbull/pkg/service/entity"
	"github.com/rs/zerolog/log"
	"github.com/shopspring/decimal"
	"golang.org/x/sync/errgroup"
)

type Request struct {
	Timestamp time.Time          `json:"timestamp" mapstructure:"timestamp"`
	Symbols   []string           `json:"symbols" mapstructure:"symbols"`
	Portfolio *finance.Portfolio `json:"portfolio" mapstructure:"portfolio"`
}

type Pool interface {
	GetPort() int
	GetNamespacePort() int
	SetAlgorithm(algo *entity.Algorithm) error
	Process(ctx context.Context, timestamp time.Time, symbols []string, portfolio *finance.Portfolio) (*[]finance.Order, error)
	Close() error
}

func NewPool(ctx context.Context, algo *entity.Algorithm) (Pool, error) {
	s, err := socket.NewRequester("0.0.0.0", 0, false)
	if err != nil {
		return nil, fmt.Errorf("error creating requester: %w", err)
	}
	namespaceSocket, err := socket.NewReplier("0.0.0.0", 0, false)
	if err != nil {
		return nil, fmt.Errorf("error creating replier: %w", err)
	}

	var n *namespace
	if algo != nil {
		n = CreateNamespace(algo.Namespaces)
	}

	p := &pool{Socket: s, NamespaceSocket: namespaceSocket, algo: algo, namespace: n}
	go p.startNamespaceListener()
	return p, nil
}

type pool struct {
	Socket          socket.Requester `json:"socket"`
	NamespaceSocket socket.Replier   `json:"namespace_socket"`

	algo      *entity.Algorithm
	namespace *namespace
}

func (p *pool) SetAlgorithm(algo *entity.Algorithm) error {
	n := CreateNamespace(algo.Namespaces)
	p.algo = algo
	p.namespace = n
	return nil
}

func (p *pool) GetPort() int {
	return p.Socket.GetPort()
}

func (p *pool) GetNamespacePort() int {
	return p.NamespaceSocket.GetPort()
}

func (p *pool) startNamespaceListener() {
	for {
		request := service_pb.NamespaceRequest{}
		response := service_pb.NamespaceResponse{}
		sock, err := p.NamespaceSocket.Recieve(&request)
		if err != nil {
			if err == socket.Closed {
				log.Info().Msg("namespace socket closed")
				return
			}
			log.Error().Err(err).Msg("error reading from namespace socket")
			continue
		}
		switch request.Type {
		case service_pb.NamespaceRequestType_GET:
			value := p.namespace.Get(request.Key)
			response.Value = value
		case service_pb.NamespaceRequestType_SET:
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
func (p *pool) orderedFunctions() <-chan entity.AlgorithmFunction {
	ch := make(chan entity.AlgorithmFunction)
	go func() {
		for _, function := range p.algo.Functions {
			if function.RunFirst && !function.RunLast {
				ch <- function
			}
		}
		for _, function := range p.algo.Functions {
			if !function.RunFirst && !function.RunLast {
				ch <- function
			}
		}
		for _, function := range p.algo.Functions {
			if !function.RunFirst && function.RunLast {
				ch <- function
			}
		}
		close(ch)
	}()
	return ch
}

func (p *pool) Process(ctx context.Context, timestamp time.Time, symbols []string, portfolio *finance.Portfolio) (*[]finance.Order, error) {
	if p.algo == nil {
		return nil, fmt.Errorf("algorithm not set")
	}
	p.namespace.Flush()

	portfolio_pb := finance_pb.Portfolio{
		Cash:  portfolio.Cash.InexactFloat64(),
		Value: portfolio.Value.InexactFloat64(),
	}
	for _, pos := range portfolio.Positions {
		portfolio_pb.Positions = append(portfolio_pb.Positions, &finance_pb.Position{
			Symbol: pos.Symbol,
			Amount: pos.Amount.InexactFloat64(),
			Cost:   pos.CostBasis.InexactFloat64(),
		})
	}

	var orders []finance.Order
	functions := p.orderedFunctions()
	for function := range functions {
		if function.ParallelExecution {
			g, _ := errgroup.WithContext(ctx)
			orderWriteMutex := sync.Mutex{}
			for _, symbol := range symbols {
				s := symbol
				g.Go(func() error {
					request := service_pb.WorkerRequest{
						Task:      function.Name,
						Timestamp: pb.TimeToProtoTimestamp(timestamp),
						Symbols:   []string{s},
						Portfolio: &portfolio_pb,
					}
					response := service_pb.WorkerResponse{}
					err := p.Socket.Request(&request, &response)
					if err != nil {
						return fmt.Errorf("error processing request: %w", err)
					}

					orderWriteMutex.Lock()
					defer orderWriteMutex.Unlock()
					for _, order := range response.Orders {
						amount := decimal.NewFromInt32(order.Amount)
						orders = append(orders, finance.Order{
							Symbol: order.Symbol,
							Amount: &amount,
						})
					}
					return nil
				})
			}
			err := g.Wait()
			if err != nil {
				return nil, err
			}
		} else {
			request := service_pb.WorkerRequest{
				Task:      function.Name,
				Timestamp: pb.TimeToProtoTimestamp(timestamp),
				Symbols:   symbols,
				Portfolio: &portfolio_pb,
			}
			response := service_pb.WorkerResponse{}
			err := p.Socket.Request(&request, &response)
			if err != nil {
				return nil, fmt.Errorf("error processing request: %w", err)
			}
			for _, order := range response.Orders {
				amount := decimal.NewFromInt32(order.Amount)
				orders = append(orders, finance.Order{
					Symbol: order.Symbol,
					Amount: &amount,
				})
			}
		}
	}
	return &orders, nil
}

func (p *pool) Close() error {
	if p.Socket != nil {
		err := p.Socket.Close()
		if err != nil && err != socket.Closed {
			return err
		}
	}
	if p.NamespaceSocket != nil {
		err := p.NamespaceSocket.Close()
		if err != nil && err != socket.Closed {
			return err
		}
	}
	return nil
}
