package socket

import (
	"context"
	"fmt"
)

type SocketType string

type Reader interface {
	Read() ([]byte, error)
	Close() error
}

type Writer interface {
	Write([]byte) error
	Close() error
}

type ReadWriter interface {
	Reader
	Writer
}

type ContextSocket interface {
	Get() (ReadWriter, error)
	Close() error
}

const (
	Requester        = SocketType("Requester")
	Replier          = SocketType("Replier")
	Publisher        = SocketType("Publisher")
	Subscriber       = SocketType("Subscriber")
	ContextRequester = SocketType("ContextRequester")
)

type Socket struct {
	Type   SocketType `json:"type"`
	Host   string     `json:"host"`
	Port   int        `json:"port"`
	Listen bool       `json:"listen"`
	Dial   bool       `json:"dial"`
}

func GetContextSocket(ctx context.Context, s *Socket) (ContextSocket, error) {
	var socket NanomsgSocket
	if s.Dial {
		socket = NanomsgSocket{SocketType: s.Type, Host: s.Host, Port: s.Port, Dial: true, Listen: false}
	} else {
		socket = NanomsgSocket{SocketType: s.Type, Host: s.Host, Port: s.Port, Dial: false, Listen: true}
	}
	if err := socket.Connect(); err != nil {
		return nil, fmt.Errorf("error connecting to socket: %v", err)
	}
	s.Port = socket.Port
	return &socket, nil
}

func GetSubscriberSocket(ctx context.Context, s *Socket) (Reader, error) {
	socket := NanomsgSocket{SocketType: Subscriber, Host: s.Host, Port: s.Port, Dial: s.Dial, Listen: s.Listen}
	err := socket.Connect()
	if err != nil {
		return nil, err
	}
	sock := Context{ctx: socket.socket}
	return &sock, nil
}
