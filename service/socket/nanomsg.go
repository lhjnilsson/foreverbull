package socket

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/lhjnilsson/foreverbull/internal/environment"
	"go.nanomsg.org/mangos/v3"
	"go.nanomsg.org/mangos/v3/protocol/pub"
	"go.nanomsg.org/mangos/v3/protocol/rep"
	"go.nanomsg.org/mangos/v3/protocol/req"
	"go.nanomsg.org/mangos/v3/protocol/sub"

	// Needed for Mangos to get needed meta- data
	_ "go.nanomsg.org/mangos/v3/transport/all"
)

/*
NanomsgSocket
containes a raw mangosSocket that is constructed during connection.
Also contains various configuration patterns used for construction
*/
type NanomsgSocket struct {
	socket      mangos.Socket `json:"-"`
	SocketType  SocketType    `json:"socket_type"`
	Host        string        `json:"host"`
	Port        int           `json:"port"`
	Dial        bool          `json:"dial"`
	Listen      bool          `json:"listen"`
	RecvTimeout int           `json:"recv_timeout"`
	SEndout     int           `json:"sendout"`
}

func (s *NanomsgSocket) listenToFreePort() error {
	var err error

	for i := environment.GetBacktestPortRangeStart(); i <= environment.GetBacktestPortRangeEnd(); i++ {
		s.Port = i
		err = s.socket.Listen(fmt.Sprintf("tcp://%v:%v", s.Host, s.Port))
		if err == nil {
			return nil
		}
		if strings.Compare(errors.Unwrap(err).Error(), "bind: address already in use") == 0 {
			continue
		}
		return err
	}
	return errors.New("no free ports in range")
}

/*
Connect
creates a socket based on the SocketType.
setting timeouts if specified.
*/
func (s *NanomsgSocket) Connect() error {
	var err error

	switch s.SocketType {
	case "Publisher":
		s.socket, err = pub.NewSocket()
	case "Subscriber":
		s.socket, err = sub.NewSocket()
		if err != nil {
			return fmt.Errorf("error creating socket: %v", err)
		}
		err = s.socket.SetOption(mangos.OptionSubscribe, []byte(""))
	case "Requester":
		s.socket, err = req.NewSocket()
	case "Replier":
		s.socket, err = rep.NewSocket()
	}
	if err != nil {
		return fmt.Errorf("error creating socket: %v", err)
	}

	/*
		if s.RecvTimeout == 0 {
			s.RecvTimeout = 10 // default to 10 seconds
		}
		if s.SEndout == 0 {
			s.SEndout = 10 // default to 10 seconds
		}
		if s.SocketType != "Publisher" && s.SocketType != "Subscriber" {
			err = s.socket.SetOption(mangos.OptionRecvDeadline, time.Second*time.Duration(s.RecvTimeout))
			if err != nil {
				return fmt.Errorf("error setting recv timeout: %v", err)
			}
			err = s.socket.SetOption(mangos.OptionSendDeadline, time.Second*time.Duration(s.SEndout))
			if err != nil {
				return fmt.Errorf("error setting send timeout: %v", err)
			}
		}
	*/
	if s.Dial {
		// try to connect 20 times, with a 1/10 second delay between each
		for i := 0; i < 20; i++ {
			err = s.socket.Dial(fmt.Sprintf("tcp://%v:%v", s.Host, s.Port))
			if err == nil {
				break
			}
			time.Sleep(time.Second / 10)
		}
	} else if s.Listen {
		if s.Port == 0 {
			err = s.listenToFreePort()
		} else {
			err = s.socket.Listen(fmt.Sprintf("tcp://%v:%v", s.Host, s.Port))
		}
	} else {
		return fmt.Errorf("Socket must be either dial or listen")
	}

	if err != nil {
		return fmt.Errorf("error connecting to socket: %v", err)
	}
	return nil
}

/*
OpenContext
context socket can be used when duing parallel requests over a socket
and wish to get responses back to match the request
*/
func (s *NanomsgSocket) Get() (ReadWriter, error) {
	socket, err := s.socket.OpenContext()
	if err != nil {
		return nil, fmt.Errorf("error opening context: %v", err)
	}
	err = socket.SetOption(mangos.OptionRecvDeadline, time.Second*time.Duration(s.RecvTimeout))
	if err != nil {
		return nil, fmt.Errorf("error setting recv timeout: %v", err)
	}
	err = socket.SetOption(mangos.OptionSendDeadline, time.Second*time.Duration(s.SEndout))
	if err != nil {
		return nil, fmt.Errorf("error setting send timeout: %v", err)
	}
	ci := Context{ctx: socket}
	return &ci, err
}

func (s *NanomsgSocket) Close() error {
	return s.socket.Close()
}

type Context struct {
	ctx mangos.Context
}

/*
Read
recieves bytes from socket.
Goes from Recv to Read to match io.ReadWriteCloser interface
*/
func (s *Context) Read() ([]byte, error) {
	return s.ctx.Recv()
}

/*
Write
Sends bytes over socket
Goes from Send to Write to match io.ReadWriteCloser interface
*/
func (s *Context) Write(msg []byte) error {
	return s.ctx.Send(msg)
}

/*
Close
Closes the underlying socket
*/
func (s *Context) Close() error {
	return s.ctx.Close()
}
