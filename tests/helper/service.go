package helper

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.nanomsg.org/mangos/v3"
	"go.nanomsg.org/mangos/v3/protocol/rep"
	_ "go.nanomsg.org/mangos/v3/transport/all"
)

type ServiceInstance struct {
	socket mangos.Socket
	Host   string
	Port   int
}

type InstanceRequest struct {
	Task string `json:"task"`
}

func NewServiceInstance(t *testing.T) *ServiceInstance {
	t.Helper()
	var err error
	var socket mangos.Socket
	host := "127.0.0.1"

	socketStart := 6800
	for ; socketStart < 6900; socketStart++ {
		socket, err = rep.NewSocket()
		if err != nil {
			t.Logf("could not create socket: %v", err)
			continue
		}
		err = socket.Listen(fmt.Sprintf("tcp://%v:%v", host, socketStart))
		if err != nil {
			socket = nil
			if strings.Compare(errors.Unwrap(err).Error(), "bind: address already in use") == 0 {
				continue
			}
			t.Fatalf("could not listen: %v", err)
			return nil
		}
		require.NoError(t, socket.SetOption(mangos.OptionSendDeadline, time.Second))
		break
	}
	if socket == nil {
		t.Fatalf("could not create socket")
		return nil
	}
	return &ServiceInstance{
		socket: socket,
		Host:   host,
		Port:   socketStart,
	}
}

func (s *ServiceInstance) Process(ctx context.Context, responses map[string][]byte) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			msg, err := s.socket.Recv()
			if err != nil {
				if err == mangos.ErrRecvTimeout || err == mangos.ErrClosed {
					continue
				} else {
					panic(err)
				}
			}
			req := InstanceRequest{}
			err = json.Unmarshal(msg, &req)
			if err != nil {
				panic(err)
			}
			rsp, ok := responses[req.Task]
			if !ok {
				panic(fmt.Sprintf("no response for task %v", req.Task))
			}

			err = s.socket.Send(rsp)
			if err != nil {
				panic(err)
			}
		}
	}
}

func (s *ServiceInstance) Close() error {
	if s.socket == nil {
		return nil
	}
	s.socket.Close()
	return nil
}
