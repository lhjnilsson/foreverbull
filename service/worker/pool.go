package worker

import (
	"context"
	"fmt"
	"time"

	finance "github.com/lhjnilsson/foreverbull/finance/entity"
	"github.com/lhjnilsson/foreverbull/internal/config"
	"github.com/lhjnilsson/foreverbull/service/entity"
	"github.com/lhjnilsson/foreverbull/service/message"
	"github.com/lhjnilsson/foreverbull/service/socket"
	"golang.org/x/sync/errgroup"
)

type Pool interface {
	SocketConfig() *socket.Socket
	ConfigureExecution(context.Context, *Configuration) error
	RunExecution(context.Context) error
	Process(context.Context, string, time.Time, string) (*finance.Order, error)
	StopExecution(context.Context) error
	Stop(context.Context) error
}

func NewPool(ctx context.Context, serverConfig *config.Config, instances ...*entity.Instance) (Pool, error) {
	var workerInstances []*Instance
	for _, instance := range instances {
		workerInstances = append(workerInstances, &Instance{Service: instance})
	}
	s := &socket.Socket{Type: socket.Requester, Host: "0.0.0.0", Port: 0, Dial: false}
	socket, err := socket.GetContextSocket(ctx, s)
	if err != nil {
		return nil, err
	}
	return &pool{Workers: workerInstances, serverConfig: serverConfig, Socket: s, socket: socket}, nil
}

type pool struct {
	serverConfig *config.Config       `json:"-"`
	Socket       *socket.Socket       `json:"socket"`
	socket       socket.ContextSocket `json:"-"`
	Workers      []*Instance          `json:"workers"`
}

func (p *pool) SocketConfig() *socket.Socket {
	cfg := p.Socket
	cfg.Host = p.serverConfig.Hostname
	return cfg
}

func (p *pool) ConfigureExecution(ctx context.Context, configuration *Configuration) error {
	if len(p.Workers) == 0 {
		return fmt.Errorf("no workers")
	}

	g, gctx := errgroup.WithContext(ctx)
	for _, worker := range p.Workers {
		w := worker
		g.Go(func() error {
			if err := w.ConfigureExecution(gctx, configuration); err != nil {
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

func (p *pool) Process(ctx context.Context, execution string, timestamp time.Time, symbol string) (*finance.Order, error) {
	context, err := p.socket.Get()
	if err != nil {
		return nil, err
	}
	defer context.Close()
	request := &message.Request{Task: "period", Data: Request{Execution: execution, Timestamp: timestamp, Symbol: symbol}}
	rsp, err := request.Process(context)
	if err != nil {
		return nil, fmt.Errorf("error processing request: %w", err)
	}
	if rsp.Data == nil {
		return nil, nil
	}
	order := finance.Order{}
	err = rsp.DecodeData(&order)
	if err != nil {
		return nil, fmt.Errorf("error decoding response: %w", err)
	}
	return &order, nil
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

	return g.Wait()
}
