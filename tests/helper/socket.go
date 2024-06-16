package helper

import (
	"fmt"
	"testing"

	"github.com/lhjnilsson/foreverbull/pkg/service/message"
	"github.com/stretchr/testify/assert"
	"go.nanomsg.org/mangos/v3"
)

func SocketRequest(t *testing.T, socket mangos.Socket, task string, data interface{}, response interface{}) error {
	t.Helper()
	req := message.Request{Task: task, Data: data}
	msg, err := req.Encode()
	if err != nil {
		return err
	}
	err = socket.Send(msg)
	if err != nil {
		return err
	}
	msg, err = socket.Recv()
	if err != nil {
		return err
	}
	rsp := message.Response{}
	err = rsp.Decode(msg)
	if err != nil {
		return err
	}
	if rsp.Error != "" {
		return fmt.Errorf("error: %s", rsp.Error)
	}
	if response != nil {
		err = rsp.DecodeData(response)
		assert.Nil(t, err)
	}
	return nil
}

func SocketReplier(t *testing.T, socket mangos.Socket, replier func(interface{}) (interface{}, error)) error {
	t.Helper()
	for {
		msg, err := socket.Recv()
		if err != nil && err.Error() == "object closed" {
			return nil
		}
		assert.Nil(t, err)
		req := message.Request{}
		err = req.Decode(msg)
		assert.Nil(t, err)
		rsp := message.Response{Task: req.Task}
		rsp.Data, err = replier(req.Data)
		assert.Nil(t, err)
		msg, err = rsp.Encode()
		assert.Nil(t, err)
		err = socket.Send(msg)
		assert.Nil(t, err)
	}
}
