package test_helper

import (
	"testing"

	"github.com/lhjnilsson/foreverbull/pkg/service/pb"
	service_pb "github.com/lhjnilsson/foreverbull/pkg/service/pb"
	"github.com/stretchr/testify/require"
	"go.nanomsg.org/mangos/v3"
	"google.golang.org/protobuf/proto"
)

func Example(req *service_pb.WorkerRequest) *service_pb.WorkerResponse {
	return &service_pb.WorkerResponse{}
}

type WorkerFunction struct {
	CB       func(*service_pb.WorkerRequest) *service_pb.WorkerResponse
	Name     string
	Parallel bool
	RunFirst bool
	RunLast  bool
}

func WorkerSimulator(t *testing.T, functions ...*WorkerFunction) (*pb.Algorithm, func(socket mangos.Socket)) {
	t.Helper()
	algo := &pb.Algorithm{
		FilePath: "worker_simulator",
	}
	for _, f := range functions {
		algo.Functions = append(algo.Functions, &pb.Algorithm_Function{
			Name:              f.Name,
			ParallelExecution: f.Parallel,
			RunFirst:          f.RunFirst,
			RunLast:           f.RunLast,
			Parameters:        []*service_pb.Algorithm_FunctionParameter{},
		})
	}
	callbacks := make(map[string]func(*service_pb.WorkerRequest) *service_pb.WorkerResponse)
	for _, f := range functions {
		callbacks[f.Name] = f.CB
	}
	runner := func(socket mangos.Socket) {
		for {
			msg, err := socket.Recv()
			if err != nil && err.Error() == "object closed" {
				break
			}
			require.NoError(t, err, "failed to receive message")
			req := service_pb.WorkerRequest{}
			err = proto.Unmarshal(msg, &req)
			require.NoError(t, err, "failed to unmarshal request")
			cb, ok := callbacks[req.Task]
			require.True(t, ok, "unknown function name")
			rsp := cb(&req)
			data, err := proto.Marshal(rsp)
			require.NoError(t, err, "failed to marshal response data")
			require.NoError(t, socket.Send(data), "failed to send response")
			t.Log("Sent response")
		}
	}
	return algo, runner
}
