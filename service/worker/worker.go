package worker

import (
	"context"
	"fmt"
	"time"

	finance "github.com/lhjnilsson/foreverbull/finance/entity"
	"github.com/lhjnilsson/foreverbull/service/entity"
	"github.com/lhjnilsson/foreverbull/service/message"
	"github.com/lhjnilsson/foreverbull/service/socket"
)

type Instance struct {
	Service *entity.Instance     `json:"-"`
	socket  socket.ContextSocket `json:"-"`
	Running bool                 `json:"running"`
}

type Request struct {
	Execution string             `json:"execution"`
	Timestamp time.Time          `json:"timestamp"`
	Symbol    string             `json:"symbol"`
	Portfolio *finance.Portfolio `json:"portfolio"`
}

type Parameter struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type Configuration struct {
	Execution  string      `json:"execution"`
	Port       int         `json:"port"`
	Parameters []Parameter `json:"parameters"`
	Database   string      `json:"database"`
}

func (i *Instance) ConfigureExecution(ctx context.Context, configuration *Configuration) error {
	var err error
	if i.socket == nil {
		s, err := i.Service.GetSocket()
		if err != nil {
			return fmt.Errorf("error getting socket for instance: %w", err)
		}
		i.socket, err = socket.GetContextSocket(ctx, s)
		if err != nil {
			return fmt.Errorf("error getting socket for instance: %w", err)
		}
	}
	socket, err := i.socket.Get()
	if err != nil {
		return fmt.Errorf("error opening context: %w", err)
	}
	defer socket.Close()
	req := message.Request{Task: "configure_execution", Data: configuration}
	rsp, err := req.Process(socket)
	if err != nil {
		return fmt.Errorf("error configuring instance: %w", err)
	}
	if rsp.HasError() {
		return fmt.Errorf("error configuring instance: %s", rsp.Error)
	}
	return nil
}

func (i *Instance) RunExecution(ctx context.Context) error {
	socket, err := i.socket.Get()
	if err != nil {
		return fmt.Errorf("error opening context: %w", err)
	}
	defer socket.Close()
	req := message.Request{Task: "run_execution"}
	rsp, err := req.Process(socket)
	if err != nil {
		return fmt.Errorf("error running instance: %w", err)
	}
	if rsp.HasError() {
		return fmt.Errorf("error running instance: %s", rsp.Error)
	}
	return nil
}

func (i *Instance) StopExecution(ctx context.Context) error {
	socket, err := i.socket.Get()
	if err != nil {
		return fmt.Errorf("error opening context: %w", err)
	}
	defer socket.Close()
	req := message.Request{Task: "stop_execution"}
	rsp, err := req.Process(socket)
	if err != nil {
		return fmt.Errorf("error stopping execution: %w", err)
	}
	if rsp.HasError() {
		return fmt.Errorf("error stopping execution: %s", rsp.Error)
	}
	return nil
}

/*
Stop
Send request to worker that the execution is over.
Expecting the worker to reply and then terminate safely
*/
func (i *Instance) Stop(ctx context.Context) error {
	if i.socket == nil {
		return nil
	}
	socket, err := i.socket.Get()
	if err != nil {
		return fmt.Errorf("error opening context: %w", err)
	}
	defer socket.Close()

	req := message.Request{Task: "stop"}
	_, err = req.Process(socket)
	i.socket = nil
	return err
}
