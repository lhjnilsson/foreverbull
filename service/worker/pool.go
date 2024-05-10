package worker

import (
	"context"
	"fmt"
	"sync"
	"time"

	finance "github.com/lhjnilsson/foreverbull/finance/entity"
	"github.com/lhjnilsson/foreverbull/service/entity"
	"github.com/lhjnilsson/foreverbull/service/message"
	"github.com/lhjnilsson/foreverbull/service/socket"
	"golang.org/x/sync/errgroup"
)

type Request struct {
	Timestamp time.Time          `json:"timestamp"`
	Symbols   []string           `json:"symbols"`
	Portfolio *finance.Portfolio `json:"portfolio"`
}

type Pool interface {
	GetPort() int
	SetAlgorithm(algo *entity.Algorithm)
	Process(ctx context.Context, timestamp time.Time, symbols []string, portfolio *finance.Portfolio) (*[]finance.Order, error)
}

func NewPool(ctx context.Context, algo *entity.Algorithm) (Pool, error) {
	s := &socket.Socket{Type: socket.Requester, Host: "0.0.0.0", Port: 0, Dial: false}
	socket, err := socket.GetContextSocket(ctx, s)
	if err != nil {
		return nil, err
	}
	return &pool{Socket: s, socket: socket, algo: algo}, nil
}

type pool struct {
	Socket *socket.Socket       `json:"socket"`
	socket socket.ContextSocket `json:"-"`

	algo *entity.Algorithm
}

func (p *pool) SetAlgorithm(algo *entity.Algorithm) {
	p.algo = algo
}

func (p *pool) GetPort() int {
	return p.Socket.Port
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
					request := &message.Request{Task: function.Name, Data: Request{Timestamp: timestamp, Symbols: []string{s}, Portfolio: portfolio}}
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
			request := &message.Request{Task: function.Name, Data: Request{Timestamp: timestamp, Symbols: symbols, Portfolio: portfolio}}
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
