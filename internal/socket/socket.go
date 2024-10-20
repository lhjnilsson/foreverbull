package socket

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/rs/zerolog/log"
	"go.nanomsg.org/mangos/v3"
	"go.nanomsg.org/mangos/v3/protocol/rep"
	"go.nanomsg.org/mangos/v3/protocol/req"
	"go.nanomsg.org/mangos/v3/protocol/sub"
	"google.golang.org/protobuf/proto"

	// Needed for Mangos to get needed meta- data
	_ "go.nanomsg.org/mangos/v3/transport/all"
)

var (
	ErrClosed      = errors.New("socket closed")
	ErrReadTimeout = errors.New("read timeout")
	ErrSendTimeout = errors.New("send timeout")
)

func sockError(err error) error {
	switch err {
	case mangos.ErrClosed:
		return ErrClosed
	case mangos.ErrRecvTimeout:
		return ErrReadTimeout
	case mangos.ErrSendTimeout:
		return ErrSendTimeout
	default:
		return fmt.Errorf("socket error: %w", err)
	}
}

type OptionSetter interface {
	SetOption(name string, value interface{}) error
}

func WithSendTimeout(t time.Duration) func(OptionSetter) error {
	return func(o OptionSetter) error {
		return o.SetOption(mangos.OptionSendDeadline, t)
	}
}

func WithReadTimeout(t time.Duration) func(OptionSetter) error {
	return func(o OptionSetter) error {
		return o.SetOption(mangos.OptionRecvDeadline, t)
	}
}

type Options struct {
	SendTimeout time.Duration
	ReadTimeout time.Duration
}

type Base interface {
	GetHost() string
	GetPort() int
	Close() error
}

type Requester interface {
	Base
	Request(msg proto.Message, reply proto.Message, opts ...func(OptionSetter) error) error
}

func NewRequester(host string, port int, dial bool, options ...func(OptionSetter) error) (Requester, error) {
	req, err := req.NewSocket()
	if err != nil {
		return nil, fmt.Errorf("failed to create requester socket: %w", err)
	}

	if dial {
		err = req.Dial(fmt.Sprintf("tcp://%s:%d", host, port))
		if err != nil {
			return nil, fmt.Errorf("failed to dial: %w", err)
		}
	} else {
		if port == 0 {
			port, err = ListenToFreePort(req, host)
			if err != nil {
				return nil, fmt.Errorf("failed to listen to free port: %w", err)
			}
		} else {
			err = req.Listen(fmt.Sprintf("tcp://%s:%d", host, port))
			if err != nil {
				return nil, fmt.Errorf("failed to listen: %w", err)
			}
		}
	}

	for _, opt := range options {
		if err := opt(req); err != nil {
			return nil, fmt.Errorf("failed to set option: %w", err)
		}
	}

	return &requester{socket: req, host: host, port: port}, nil
}

type requester struct {
	socket mangos.Socket
	host   string
	port   int
}

func (r *requester) GetHost() string {
	return r.host
}

func (r *requester) GetPort() int {
	return r.port
}

func (r *requester) Close() error {
	if err := r.socket.Close(); err != nil {
		return sockError(err)
	}

	return nil
}

func (r *requester) Request(msg proto.Message, reply proto.Message, options ...func(OptionSetter) error) error {
	bytes, err := proto.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	ctx, err := r.socket.OpenContext()
	if err != nil {
		return sockError(err)
	}

	defer ctx.Close()

	for _, opt := range options {
		if err := opt(ctx); err != nil {
			return fmt.Errorf("failed to set option: %w", err)
		}
	}

	if err := ctx.Send(bytes); err != nil {
		return sockError(err)
	}

	bytes, err = ctx.Recv()
	if err != nil {
		return sockError(err)
	}

	if reply != nil {
		if err = proto.Unmarshal(bytes, reply); err != nil {
			return fmt.Errorf("failed to unmarshal response: %w", err)
		}
	}

	return nil
}

type Replier interface {
	Base
	Recieve(proto.Message, ...func(OptionSetter) error) (ReplierSocket, error)
}

type ReplierSocket interface {
	Reply(proto.Message, ...func(OptionSetter) error) error
}

func NewReplier(host string, port int, dial bool, options ...func(OptionSetter) error) (Replier, error) {
	rep, err := rep.NewSocket()
	if err != nil {
		return nil, fmt.Errorf("failed to create replier socket: %w", err)
	}

	if dial {
		err = rep.Dial(fmt.Sprintf("tcp://%s:%d", host, port))
		if err != nil {
			return nil, fmt.Errorf("failed to dial: %w", err)
		}
	} else {
		if port == 0 {
			port, err = ListenToFreePort(rep, host)
			if err != nil {
				return nil, fmt.Errorf("failed to listen: %w", err)
			}
		} else {
			err = rep.Listen(fmt.Sprintf("tcp://%s:%d", host, port))
			if err != nil {
				return nil, fmt.Errorf("failed to listen: %w", err)
			}
		}
	}

	for _, opt := range options {
		if err := opt(rep); err != nil {
			return nil, fmt.Errorf("failed to set option: %w", err)
		}
	}

	return &replier{socket: rep, host: host, port: port}, nil
}

type replier struct {
	socket mangos.Socket
	host   string
	port   int
}

func (r *replier) GetHost() string {
	return r.host
}

func (r *replier) GetPort() int {
	return r.port
}

func (r *replier) Close() error {
	if err := r.socket.Close(); err != nil {
		return sockError(err)
	}

	return nil
}

type replierSocket struct {
	socket mangos.Context
}

func (r *replierSocket) Reply(msg proto.Message, options ...func(OptionSetter) error) error {
	defer r.socket.Close()

	for _, opt := range options {
		if err := opt(r.socket); err != nil {
			return fmt.Errorf("failed to set option: %w", err)
		}
	}

	bytes, err := proto.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal response: %w", err)
	}

	if err := r.socket.Send(bytes); err != nil {
		return fmt.Errorf("failed to send response: %w", err)
	}

	return nil
}

func (r *replier) Recieve(msg proto.Message, options ...func(OptionSetter) error) (ReplierSocket, error) {
	ctx, err := r.socket.OpenContext()
	if err != nil {
		return nil, sockError(err)
	}

	for _, opt := range options {
		if err := opt(ctx); err != nil {
			return nil, fmt.Errorf("failed to set option: %w", err)
		}
	}

	bytes, err := ctx.Recv()
	if err != nil {
		return nil, sockError(err)
	}

	if err := proto.Unmarshal(bytes, msg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal request: %w", err)
	}

	replier := &replierSocket{socket: ctx}

	return replier, nil
}

type Subscriber interface {
	Base
	Recieve(proto.Message, ...func(OptionSetter) error) error
}

func NewSubscriber(host string, port int, options ...func(OptionSetter) error) (Subscriber, error) {
	sub, err := sub.NewSocket()
	if err != nil {
		return nil, fmt.Errorf("failed to create subscriber socket: %w", err)
	}

	err = sub.Dial(fmt.Sprintf("tcp://%s:%d", host, port))
	if err != nil {
		return nil, fmt.Errorf("failed to dial: %w", err)
	}

	err = sub.SetOption(mangos.OptionSubscribe, []byte(""))
	if err != nil {
		return nil, fmt.Errorf("failed to subscribe: %w", err)
	}

	for _, opt := range options {
		if err := opt(sub); err != nil {
			return nil, fmt.Errorf("failed to set option: %w", err)
		}
	}

	return &subscriber{socket: sub}, nil
}

type subscriber struct {
	socket mangos.Socket
	host   string
	port   int
}

func (s *subscriber) GetHost() string {
	return s.host
}

func (s *subscriber) GetPort() int {
	return s.port
}

func (s *subscriber) Close() error {
	if err := s.socket.Close(); err != nil {
		return sockError(err)
	}

	return nil
}

func (s *subscriber) Recieve(msg proto.Message, options ...func(OptionSetter) error) error {
	bytes, err := s.socket.Recv()
	if err != nil {
		return fmt.Errorf("failed to receive message: %w", err)
	}

	if err := proto.Unmarshal(bytes, msg); err != nil {
		return fmt.Errorf("failed to unmarshal message: %w", err)
	}

	return nil
}

func ListenToFreePort(socket mangos.Socket, host string) (int, error) {
	var err error

	for port := environment.GetBacktestPortRangeStart(); port <= environment.GetBacktestPortRangeEnd(); port++ {
		err = socket.Listen(fmt.Sprintf("tcp://%v:%v", host, port))

		if err == nil {
			return port, nil
		}

		if strings.Compare(errors.Unwrap(err).Error(), "bind: address already in use") == 0 {
			log.Debug().Msgf("Port %v already in use, trying next port", port)
			continue
		}

		return 0, fmt.Errorf("error listening to port %v: %v", port, err)
	}

	return 0, errors.New("no free ports in range")
}
