package worker

import (
	"context"
	"fmt"
	"sync"
	"time"

	finance "github.com/lhjnilsson/foreverbull/finance/entity"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/service/entity"
	"github.com/lhjnilsson/foreverbull/service/message"
	"github.com/lhjnilsson/foreverbull/service/socket"
	"golang.org/x/sync/errgroup"
)

type Pool interface {
	SocketConfig() *socket.Socket
	ConfigureExecution(context.Context, *entity.Instance) error
	RunExecution(context.Context) error
	Process(ctx context.Context, timestamp time.Time, symbols []string, portfolio *finance.Portfolio) (*[]finance.Order, error)
	StopExecution(context.Context) error
	Stop(context.Context) error
}

func NewPool(ctx context.Context, instances ...*entity.Instance) (Pool, error) {
	var workerInstances []*Instance
	for _, instance := range instances {
		workerInstances = append(workerInstances, &Instance{Service: instance})
	}

	algoCh := make(chan *entity.Algorithm, len(instances))
	g, _ := errgroup.WithContext(ctx)
	for _, instance := range instances {
		i := instance
		g.Go(func() error {
			a, err := i.GetAlgorithm()
			if err != nil {
				return err
			}
			algoCh <- a
			return nil
		})
	}
	err := g.Wait()
	if err != nil {
		return nil, err
	}
	close(algoCh)

	var algo *entity.Algorithm
	for a := range algoCh {
		if algo == nil {
			algo = a
			continue
		}
		if a != algo {
			return nil, fmt.Errorf("inconsistent service algorithms")
		}
	}

	s := &socket.Socket{Type: socket.Requester, Host: "0.0.0.0", Port: 0, Dial: false}
	socket, err := socket.GetContextSocket(ctx, s)
	if err != nil {
		return nil, err
	}
	return &pool{Workers: workerInstances, Socket: s, socket: socket, algo: algo}, nil
}

type pool struct {
	Socket  *socket.Socket       `json:"socket"`
	socket  socket.ContextSocket `json:"-"`
	Workers []*Instance          `json:"workers"`

	algo *entity.Algorithm
}

func (p *pool) SetAlgorithm(algo *entity.Algorithm) {
	p.algo = algo
}

func (p *pool) SocketConfig() *socket.Socket {
	cfg := p.Socket
	cfg.Host = environment.GetServerAddress()
	return cfg
}

func (p *pool) ConfigureExecution(ctx context.Context, i *entity.Instance) error {
	if len(p.Workers) == 0 {
		return fmt.Errorf("no workers")
	}
	g, gctx := errgroup.WithContext(ctx)
	for _, worker := range p.Workers {
		w := worker
		g.Go(func() error {
			if err := w.ConfigureExecution(gctx, i); err != nil {
				return fmt.Errorf("error configuring worker: %w", err)
			}
			return nil
		})
	}
	err := g.Wait()
	if err != nil {
		return err
	}
	return nil
}

func (p *pool) RunExecution(ctx context.Context) error {
	g, gctx := errgroup.WithContext(ctx)
	for _, worker := range p.Workers {
		worker := worker
		g.Go(func() error {
			if err := worker.RunExecution(gctx); err != nil {
				return fmt.Errorf("error running worker: %w", err)
			}
			return nil
		})
	}
	return g.Wait()
}

func (p *pool) Process(ctx context.Context, timestamp time.Time, symbols []string, portfolio *finance.Portfolio) (*[]finance.Order, error) {
	var orders []finance.Order
	for _, function := range p.algo.Functions {
		if *function.ParallelExecution {
			g, _ := errgroup.WithContext(ctx)
			orderWriteMutex := sync.Mutex{}
			for _, symbol := range symbols {
				s := symbol
				g.Go(func() error {
					context, err := p.socket.Get()
					if err != nil {
						return err
					}
					defer context.Close()
					request := &message.Request{Task: "period", Data: Request{Timestamp: timestamp, Symbols: []string{s}, Portfolio: portfolio}}
					rsp, err := request.Process(context)
					if err != nil {
						return fmt.Errorf("error processing request: %w", err)
					}
					if rsp.Data == nil {
						return nil
					}
					orderWriteMutex.Lock()
					defer orderWriteMutex.Unlock()
					order := []finance.Order{}
					err = rsp.DecodeData(&order)
					if err != nil {
						return fmt.Errorf("error decoding response: %w", err)
					}
					orders = append(orders, order...)
					return nil
				})
			}
			err := g.Wait()
			if err != nil {
				return nil, err
			}
		} else {
			context, err := p.socket.Get()
			if err != nil {
				return nil, err
			}
			defer context.Close()
			request := &message.Request{Task: "period", Data: Request{Timestamp: timestamp, Symbols: symbols, Portfolio: portfolio}}
			rsp, err := request.Process(context)
			if err != nil {
				return nil, fmt.Errorf("error processing request: %w", err)
			}
			if rsp.Data == nil {
				return nil, nil
			}
			err = rsp.DecodeData(&orders)
			if err != nil {
				return nil, fmt.Errorf("error decoding response: %w", err)
			}
		}
	}
	return &orders, nil
}

func (p *pool) StopExecution(ctx context.Context) error {
	g, gctx := errgroup.WithContext(ctx)
	for _, worker := range p.Workers {
		worker := worker
		g.Go(func() error {
			if err := worker.StopExecution(gctx); err != nil {
				return fmt.Errorf("error stopping worker: %w", err)
			}
			return nil
		})
	}

	return g.Wait()
}

func (p *pool) Stop(ctx context.Context) error {
	p.socket.Close()
	g, gctx := errgroup.WithContext(ctx)
	for _, worker := range p.Workers {
		worker := worker
		g.Go(func() error {
			if err := worker.Stop(gctx); err != nil {
				return fmt.Errorf("error stopping worker: %w", err)
			}
			return nil
		})
	}
	defer p.socket.Close()
	return g.Wait()
}
