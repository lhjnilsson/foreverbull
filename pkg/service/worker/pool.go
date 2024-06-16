package worker

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	finance "github.com/lhjnilsson/foreverbull/pkg/finance/entity"
	"github.com/lhjnilsson/foreverbull/pkg/service/entity"
	"github.com/lhjnilsson/foreverbull/pkg/service/message"
	"github.com/lhjnilsson/foreverbull/pkg/service/socket"
	"github.com/rs/zerolog/log"
	"go.nanomsg.org/mangos/v3"
	"go.nanomsg.org/mangos/v3/protocol/rep"
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
	s := &socket.Socket{Type: socket.Requester, Host: "0.0.0.0", Port: 0, Dial: false}
	sock, err := socket.GetContextSocket(ctx, s)
	if err != nil {
		return nil, err
	}

	namespaceSocket := &socket.Socket{Type: socket.Replier, Host: "0.0.0.0", Port: 0, Dial: false}
	nSock, err := rep.NewSocket()
	if err != nil {
		log.Error().Err(err).Msg("error creating socket")
		return nil, err
	}
	namespaceSocket.Port, err = socket.ListenToFreePort(nSock, namespaceSocket.Host)
	if err != nil {
		log.Error().Err(err).Msg("error listening to free port")
		return nil, err
	}

	err = nSock.SetOption(mangos.OptionRecvDeadline, time.Second/2)
	if err != nil {
		return nil, fmt.Errorf("error setting receive deadline: %w", err)
	}
	err = nSock.SetOption(mangos.OptionSendDeadline, time.Second)
	if err != nil {
		return nil, fmt.Errorf("error setting send deadline: %w", err)
	}

	var n *namespace
	if algo != nil {
		n, err = CreateNamespace(algo)
		if err != nil {
			return nil, err
		}
	}

	p := &pool{Socket: s, socket: sock,
		NamespaceSocket: namespaceSocket, namespaceSocket: nSock,
		algo: algo, namespace: n}
	go p.startNamespaceListener()
	return p, nil
}

type pool struct {
	Socket *socket.Socket       `json:"socket"`
	socket socket.ContextSocket `json:"-"`

	NamespaceSocket *socket.Socket `json:"namespace_socket"`
	namespaceSocket mangos.Socket  `json:"-"`

	algo      *entity.Algorithm
	namespace *namespace
}

func (p *pool) SetAlgorithm(algo *entity.Algorithm) error {
	n, err := CreateNamespace(algo)
	if err != nil {
		return fmt.Errorf("error creating namespace: %w", err)
	}
	p.algo = algo
	p.namespace = n
	return nil
}

func (p *pool) GetPort() int {
	return p.Socket.Port
}

func (p *pool) GetNamespacePort() int {
	return p.NamespaceSocket.Port
}

func (p *pool) startNamespaceListener() {
	sendResponse := func(context mangos.Context, response *message.Response) {
		bytes, err := response.Encode()
		if err != nil {
			log.Error().Err(err).Msg("error encoding response")
			return
		}
		err = context.Send(bytes)
		if err != nil {
			if err == mangos.ErrClosed {
				log.Err(err).Msg("socket closed while sending response")
				return
			}
			log.Error().Err(err).Msg("error writing response")
			return
		}
		context.Close()
	}

	for {
		context, err := p.namespaceSocket.OpenContext()
		if err != nil {
			if err == mangos.ErrClosed {
				return
			}
			log.Error().Err(err).Msg("error getting context")
			continue
		}
		bytes, err := context.Recv()
		if err != nil {
			if err == mangos.ErrClosed {
				return
			}
			log.Error().Err(err).Msg("error reading from context")
			continue
		}
		request := &message.Request{}
		err = request.Decode(bytes)
		if err != nil {
			log.Error().Err(err).Msg("error decoding request")
			context.Close()
			continue
		}
		response := &message.Response{Task: request.Task}
		task := strings.Split(request.Task, ":")
		if len(task) != 2 {
			log.Warn().Msg("invalid task")
			response.Error = "invalid task"
			sendResponse(context, response)
			continue
		}
		x, n := task[0], task[1]
		switch x {
		case "get":
			value, err := p.namespace.Get(n)
			if err != nil {
				log.Warn().Msg("error getting namespace")
				response.Error = err.Error()
				sendResponse(context, response)
				continue
			}
			response.Data = value
		case "set":
			if request.Data == nil {
				log.Warn().Msg("no data in set request")
				response.Error = "no data in set request"
				sendResponse(context, response)
				continue
			}
			err := p.namespace.Set(n, request.Data)
			if err != nil {
				log.Warn().Msg("error setting namespace")
				response.Error = err.Error()
				sendResponse(context, response)
				continue
			}
		default:
			log.Warn().Msg("invalid task")
			response.Error = "invalid task"
			sendResponse(context, response)
			continue
		}
		sendResponse(context, response)
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

	var orders []finance.Order
	functions := p.orderedFunctions()
	for function := range functions {
		if function.ParallelExecution {
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
				continue
			}
			err = rsp.DecodeData(&orders)
			if err != nil {
				return nil, fmt.Errorf("error decoding response: %w", err)
			}
		}
	}
	return &orders, nil
}

func (p *pool) Close() error {
	if p.socket != nil {
		err := p.socket.Close()
		if err != nil {
			return err
		}
	}
	if p.namespaceSocket != nil {
		err := p.namespaceSocket.Close()
		if err != nil {
			return err
		}
	}
	return nil
}
