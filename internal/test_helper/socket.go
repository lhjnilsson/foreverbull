package test_helper

import (
	"testing"

	service_pb "github.com/lhjnilsson/foreverbull/internal/pb/service"
	"github.com/stretchr/testify/require"
	"go.nanomsg.org/mangos/v3"
	"google.golang.org/protobuf/proto"
)

func SocketRequest(t *testing.T, socket mangos.Socket, task string, request proto.Message, response proto.Message) {
	t.Helper()
	data, err := proto.Marshal(request)
	require.Nil(t, err, "failed to marshal request data")
	req := service_pb.Request{
		Task: task,
		Data: data,
	}
	bytes, err := proto.Marshal(&req)
	require.Nil(t, err, "failed to marshal request")
	err = socket.Send(bytes)
	require.Nil(t, err, "failed to send request")
	bytes, err = socket.Recv()
	require.Nil(t, err, "failed to receive response")
	rsp := service_pb.Response{}
	err = proto.Unmarshal(bytes, &rsp)
	require.Nil(t, err, "failed to unmarshal response")
	require.Empty(t, rsp.Error, "response error")

	if response != nil {
		err = proto.Unmarshal(rsp.Data, response)
		require.Nil(t, err, "failed to unmarshal response data")
	}
}

func SocketReplier(t *testing.T, socket mangos.Socket, replier func(interface{}) (proto.Message, error)) {
	t.Helper()
	for {
		msg, err := socket.Recv()
		if err != nil && err.Error() == "object closed" {
			break
		}
		require.Nil(t, err, "failed to receive message")
		req := service_pb.Request{}
		err = proto.Unmarshal(msg, &req)
		require.Nil(t, err, "failed to unmarshal request")
		rsp := service_pb.Response{Task: req.Task}
		rspData, err := replier(req.Data)
		require.Nil(t, err, "failed to process request")
		data, err := proto.Marshal(rspData)
		require.Nil(t, err, "failed to marshal response data")
		rsp.Data = data
		msg, err = proto.Marshal(&rsp)
		require.Nil(t, err, "failed to marshal response")
		err = socket.Send(msg)
		require.Nil(t, err, "failed to send response")
	}
}
