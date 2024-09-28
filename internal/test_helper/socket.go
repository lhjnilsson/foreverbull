package test_helper

import (
	"testing"

	common_pb "github.com/lhjnilsson/foreverbull/internal/pb"
	service_pb "github.com/lhjnilsson/foreverbull/pkg/service/pb"
	"github.com/stretchr/testify/require"
	"go.nanomsg.org/mangos/v3"
	"google.golang.org/protobuf/proto"
)

func SocketRequest(t *testing.T, socket mangos.Socket, task string, request proto.Message, response proto.Message) {
	t.Helper()
	data, err := proto.Marshal(request)
	require.Nil(t, err, "failed to marshal request data")
	req := common_pb.Request{
		Task: task,
		Data: data,
	}
	bytes, err := proto.Marshal(&req)
	require.Nil(t, err, "failed to marshal request")
	err = socket.Send(bytes)
	require.Nil(t, err, "failed to send request")
	bytes, err = socket.Recv()
	require.Nil(t, err, "failed to receive response")
	rsp := common_pb.Response{}
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
		req := common_pb.Request{}
		err = proto.Unmarshal(msg, &req)
		require.Nil(t, err, "failed to unmarshal request")
		rsp := common_pb.Response{Task: req.Task}
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

func Example(req *service_pb.WorkerRequest) *service_pb.WorkerResponse {
	return &service_pb.WorkerResponse{}
}

func WorkerSimulator(t *testing.T, socket mangos.Socket, cb func(*service_pb.WorkerRequest) *service_pb.WorkerResponse) {
	t.Helper()
	for {
		msg, err := socket.Recv()
		t.Log("Received message")
		if err != nil && err.Error() == "object closed" {
			break
		}
		require.NoError(t, err, "failed to receive message")
		req := service_pb.WorkerRequest{}
		err = proto.Unmarshal(msg, &req)
		require.NoError(t, err, "failed to unmarshal request")
		rsp := cb(&req)
		data, err := proto.Marshal(rsp)
		require.NoError(t, err, "failed to marshal response data")
		require.NoError(t, socket.Send(data), "failed to send response")
		t.Log("Sent response")
	}
}
