package entity

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/lhjnilsson/foreverbull/service/message"
	"github.com/lhjnilsson/foreverbull/service/socket"
)

type InstanceStatusType string

const (
	InstanceStatusCreated InstanceStatusType = "CREATED"
	InstanceStatusRunning InstanceStatusType = "RUNNING"
	InstanceStatusStopped InstanceStatusType = "STOPPED"
	InstanceStatusError   InstanceStatusType = "ERROR"
)

type InstanceFunction struct {
	Parameters map[string]string `json:"parameters" mapstructure:"parameters"`
}

type InstanceStatus struct {
	Status     InstanceStatusType `json:"status"`
	Error      *string            `json:"message"`
	OccurredAt time.Time          `json:"occurred_at"`
}

type Instance struct {
	ID    string  `json:"id"`
	Image *string `json:"image"`

	Host        *string                      `json:"host"`
	Port        *int                         `json:"port"`
	BrokerPort  *int                         `json:"broker_port"`
	DatabaseURL *string                      `json:"database_url"`
	Functions   *map[string]InstanceFunction `json:"functions" mapstructure:"functions"`

	Statuses []InstanceStatus `json:"statuses"`
}

func (i *Instance) GetSocket() (*socket.Socket, error) {
	if i.Host == nil || i.Port == nil {
		return nil, fmt.Errorf("instance %s has no host or port", i.ID)
	}
	return &socket.Socket{Type: socket.Requester, Host: *i.Host, Port: *i.Port, Listen: false, Dial: true}, nil
}

func (i *Instance) GetAlgorithm() (*Algorithm, error) {
	iSocket, err := i.GetSocket()
	if err != nil {
		return nil, err
	}
	s, err := socket.GetContextSocket(context.Background(), iSocket)
	if err != nil {
		return nil, err
	}
	defer s.Close()
	ctxSock, err := s.Get()
	if err != nil {
		return nil, err
	}
	defer ctxSock.Close()

	req := message.Request{Task: "info"}
	rsp, err := req.Process(ctxSock)
	if err != nil {
		return nil, err
	}
	if rsp.Error != "" {
		return nil, errors.New(rsp.Error)
	}

	algo := Algorithm{}
	if err := rsp.DecodeData(&algo); err != nil {
		return nil, err
	}
	return &algo, nil
}

func (i *Instance) Configure() error {
	iSocket, err := i.GetSocket()
	if err != nil {
		return err
	}
	s, err := socket.GetContextSocket(context.Background(), iSocket)
	if err != nil {
		return err
	}
	defer s.Close()
	ctxSock, err := s.Get()
	if err != nil {
		return err
	}
	defer ctxSock.Close()

	req := message.Request{Task: "configure_execution", Data: i}
	rsp, err := req.Process(ctxSock)
	if err != nil {
		return err
	}
	if rsp.Error != "" {
		return errors.New(rsp.Error)
	}
	req = message.Request{Task: "run_execution", Data: i}
	rsp, err = req.Process(ctxSock)
	if err != nil {
		return err
	}
	if rsp.Error != "" {
		return errors.New(rsp.Error)
	}
	return nil
}

func (i *Instance) Stop() error {
	iSocket, err := i.GetSocket()
	if err != nil {
		return err
	}
	s, err := socket.GetContextSocket(context.Background(), iSocket)
	if err != nil {
		return err
	}
	defer s.Close()
	ctxSock, err := s.Get()
	if err != nil {
		return err
	}
	defer ctxSock.Close()

	req := message.Request{Task: "stop", Data: i}
	rsp, err := req.Process(ctxSock)
	if err != nil {
		return err
	}
	if rsp.Error != "" {
		return errors.New(rsp.Error)
	}
	return nil
}
