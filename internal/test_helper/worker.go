package test_helper //nolint:stylecheck,revive

import (
	"testing"

	pb "github.com/lhjnilsson/foreverbull/pkg/pb/service"
	servicePb "github.com/lhjnilsson/foreverbull/pkg/pb/service/service"
	"github.com/stretchr/testify/require"
	"go.nanomsg.org/mangos/v3"
	"google.golang.org/protobuf/proto"
)

// Example is a no-op worker callback for tests.
func Example(_ *servicePb.WorkerRequest) *servicePb.WorkerResponse {
	return &servicePb.WorkerResponse{}
}

type WorkerFunction struct {
	CB       func(*servicePb.WorkerRequest) *servicePb.WorkerResponse
	Name     string
	Parallel bool
	RunFirst bool
	RunLast  bool
}

// WorkerSimulator returns an algorithm description and a socket runner for tests.
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
			Parameters:        []*pb.Algorithm_FunctionParameter{},
		})
	}

	callbacks := make(map[string]func(*servicePb.WorkerRequest) *servicePb.WorkerResponse)
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

			req := servicePb.WorkerRequest{}
			err = proto.Unmarshal(msg, &req)
			require.NoError(t, err, "failed to unmarshal request")

			cb, ok := callbacks[req.Task]
			require.True(t, ok, "unknown function name")

			rsp := cb(&req)
			data, err := proto.Marshal(rsp)
			require.NoError(t, err, "failed to marshal response data")
			require.NoError(t, socket.Send(data), "failed to send response")
		}
	}

	return algo, runner
}
