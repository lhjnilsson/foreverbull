package worker_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/lhjnilsson/foreverbull/internal/test_helper"
	finance_pb "github.com/lhjnilsson/foreverbull/pkg/pb/finance"
	pb "github.com/lhjnilsson/foreverbull/pkg/pb/service"
	"github.com/lhjnilsson/foreverbull/pkg/service/worker"
	"github.com/stretchr/testify/suite"
	"go.nanomsg.org/mangos/v3"
	"go.nanomsg.org/mangos/v3/protocol/rep"
	"go.nanomsg.org/mangos/v3/protocol/req"
	_ "go.nanomsg.org/mangos/v3/transport/all" // for mangos transport.
)

type PoolTest struct {
	suite.Suite
}

func (test *PoolTest) SetupSuite() {
	test_helper.SetupEnvironment(test.T(), &test_helper.Containers{})
}

func TestPool(t *testing.T) {
	suite.Run(t, new(PoolTest))
}

func (test *PoolTest) TestSimple() {
	cb := func(_ *pb.WorkerRequest) *pb.WorkerResponse {
		return &pb.WorkerResponse{}
	}
	functions := []*test_helper.WorkerFunction{
		{
			CB:       cb,
			Name:     "test",
			Parallel: true,
		},
	}
	algo, runner := test_helper.WorkerSimulator(test.T(), functions...)
	pool, err := worker.NewPool(context.TODO(), algo)
	test.Require().NoError(err)

	configuration := pool.Configure()

	socket, err := rep.NewSocket()
	test.Require().NoError(err)
	test.Require().NoError(socket.Dial(fmt.Sprintf("tcp://localhost:%d", configuration.BrokerPort)))
	test.Require().NoError(socket.SetOption(mangos.OptionRecvDeadline, time.Second))
	test.Require().NoError(socket.SetOption(mangos.OptionSendDeadline, time.Second))

	namespaceSocket, err := req.NewSocket()
	test.Require().NoError(err)
	test.Require().NoError(namespaceSocket.Dial(fmt.Sprintf("tcp://localhost:%d", configuration.NamespacePort)))

	go runner(socket)

	orders, err := pool.Process(context.TODO(), time.Now(), []string{"test"}, &finance_pb.Portfolio{})
	test.Require().NoError(err)
	test.Require().Empty(orders)

	socket.Close()
	namespaceSocket.Close()
}
