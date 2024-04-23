package worker

import (
	"context"
	"errors"
	"fmt"
	"time"

	finance "github.com/lhjnilsson/foreverbull/finance/entity"
	"github.com/lhjnilsson/foreverbull/service/entity"
	"github.com/lhjnilsson/foreverbull/service/message"
	"github.com/lhjnilsson/foreverbull/service/socket"
	"golang.org/x/sync/errgroup"
)

type WorkerPool struct {
	socket    socket.ContextSocket
	instances []socket.ContextSocket
	algorithm *entity.ServiceAlgorithm
}

type Request struct {
	Timestamp time.Time         `json:"timestamp"`
	Symbol    *string           `json:"symbol"`
	Symbols   *[]string         `json:"symbols"`
	Portfolio finance.Portfolio `json:"portfolio"`
}

func NewWorkerPool(ctx context.Context, instances ...*entity.Instance) (*WorkerPool, *socket.Socket, error) {
	s := &socket.Socket{Type: socket.Requester, Host: "0.0.0.0", Port: 0, Dial: false}
	sock, err := socket.GetContextSocket(ctx, s)
	if err != nil {
		return nil, nil, fmt.Errorf("error getting socket: %w", err)
	}

	wp := WorkerPool{
		instances: make([]socket.ContextSocket, 0),
		socket:    sock,
	}
	for _, instance := range instances {
		s, err := instance.GetSocket()
		if err != nil {
			return nil, nil, fmt.Errorf("error getting socket for instance: %w", err)
		}
		sock, err := socket.GetContextSocket(context.Background(), s)
		if err != nil {
			return nil, nil, fmt.Errorf("error getting socket for instance: %w", err)
		}
		wp.instances = append(wp.instances, sock)
	}
	return &wp, s, nil
}

func (w *WorkerPool) Configure(ctx context.Context, algorithm *entity.ServiceAlgorithm, databaseURL string) error {
	g, _ := errgroup.WithContext(ctx)
	for _, instance := range w.instances {
		g.Go(func() error {
			socket, err := instance.Get()
			if err != nil {
				return err
			}
			defer socket.Close()
			req := message.Request{
				Task: "configure_execution",
			}
			rsp, err := req.Process(socket)
			if err != nil {
				return err
			}
			if rsp.HasError() {
				return errors.New(rsp.Error)
			}
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return fmt.Errorf("error configuring instances: %w", err)
	}
	return nil
}

func (w *WorkerPool) Run(ctx context.Context) error {
	g, _ := errgroup.WithContext(ctx)
	for _, instance := range w.instances {
		g.Go(func() error {
			socket, err := instance.Get()
			if err != nil {
				return err
			}
			defer socket.Close()
			req := message.Request{
				Task: "run_execution",
			}
			rsp, err := req.Process(socket)
			if err != nil {
				return err
			}
			if rsp.HasError() {
				return errors.New(rsp.Error)
			}
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return fmt.Errorf("error running instances: %w", err)
	}
	return nil
}

func (w *WorkerPool) Process(ctx context.Context, timestamp time.Time, portfolio *finance.Portfolio, symbols []string) error {
	a := NewAlgorithm(w.algorithm)

	namespaces := make(map[string]Namespace)
	namespaces["symbols"] = &NamespaceArray[string]{data: symbols}
	namespaces["orders"] = &NamespaceArray[finance.Order]{data: make([]finance.Order, 0)}

	for k, v := range w.algorithm.Namespace {
		switch v.Type {
		case "array":
			switch v.ValueType {
			case "string":
				namespaces[k] = &NamespaceArray[string]{data: make([]string, 0)}
			default:
				return fmt.Errorf("unsupported namespace value type: %s", v.ValueType)
			}
		case "object":
			switch v.ValueType {
			case "float":
				namespaces[k] = &NamespaceObject[float64]{data: make(map[string]float64)}
			default:
				return fmt.Errorf("unsupported namespace value type: %s", v.ValueType)
			}
		}
	}

	ch := a.GetFunctionChannel()
	for f := range ch {
		namespace, found := namespaces[f.InputKey]
		if !found {
			return fmt.Errorf("namespace not found: %s", f.InputKey)
		}
		symbols := namespace.GetData().([]string)

		if f.ParallelExecution {
			g, _ := errgroup.WithContext(ctx)
			for _, symbol := range symbols {
				g.Go(func() error {
					req := message.Request{
						Task: f.Name,
						Data: Request{
							Timestamp: timestamp,
							Symbol:    &symbol,
							Portfolio: *portfolio,
						},
					}
					s, err := w.socket.Get()
					if err != nil {
						return fmt.Errorf("error getting socket: %w", err)
					}

					rsp, err := req.Process(s)
					if err != nil {
						return fmt.Errorf("error processing request: %w", err)
					}
					if rsp.HasError() {
						return fmt.Errorf("error processing instance request: %s", rsp.Error)
					}
					if rsp.Data != nil {
						err = AddToNamespace(namespaces[*f.NamespaceReturnKey], f, rsp.Data)
						if err != nil {
							return fmt.Errorf("error adding data to namespace: %w", err)
						}
					}
					return nil
				})
			}
			if err := g.Wait(); err != nil {
				return fmt.Errorf("error processing instances request: %w", err)
			}
		} else {
			req := message.Request{
				Task: f.Name,
				Data: Request{
					Timestamp: timestamp,
					Symbols:   &symbols,
					Portfolio: *portfolio,
				},
			}
			s, err := w.socket.Get()
			if err != nil {
				return fmt.Errorf("error getting socket: %w", err)
			}
			rsp, err := req.Process(s)
			if err != nil {
				return fmt.Errorf("error processing request: %w", err)
			}
			if rsp.HasError() {
				return fmt.Errorf("error processing instance request: %s", rsp.Error)
			}
			if rsp.Data != nil {
				err = AddToNamespace(namespaces[*f.NamespaceReturnKey], f, rsp.Data)
				if err != nil {
					return fmt.Errorf("error adding data to namespace: %w", err)
				}
			}
		}
	}
	return nil
}

func (w *WorkerPool) Stop(ctx context.Context) error {
	g, _ := errgroup.WithContext(ctx)
	for _, instance := range w.instances {
		g.Go(func() error {
			socket, err := instance.Get()
			if err != nil {
				return err
			}
			defer socket.Close()
			req := message.Request{
				Task: "stop_execution",
			}
			rsp, err := req.Process(socket)
			if err != nil {
				return err
			}
			if rsp.HasError() {
				return errors.New(rsp.Error)
			}
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return fmt.Errorf("error stopping instances: %w", err)
	}
	return nil
}
