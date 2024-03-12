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

type Instance struct {
	ID    string  `json:"id"`
	Image string  `json:"image"`
	Host  *string `json:"host"`
	Port  *int    `json:"port"`

	Statuses []InstanceStatus `json:"statuses"`
}

type InstanceStatus struct {
	Status     InstanceStatusType `json:"status"`
	Error      *string            `json:"message"`
	OccurredAt time.Time          `json:"occurred_at"`
}

func (i *Instance) GetSocket() (*socket.Socket, error) {
	if i.Host == nil || i.Port == nil {
		return nil, fmt.Errorf("instance %s has no host or port", i.ID)
	}
	return &socket.Socket{Type: socket.Requester, Host: *i.Host, Port: *i.Port, Listen: false, Dial: true}, nil
}

func (i *Instance) GetInfo() (*Service, error) {
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

	service := Service{}
	if err := rsp.DecodeData(&service); err != nil {
		return nil, err
	}
	return &service, nil
}
