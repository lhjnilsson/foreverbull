package worker

import (
	"context"
	"fmt"
	"testing"
	"time"

	finance_pb "github.com/lhjnilsson/foreverbull/pkg/finance/pb"

	"github.com/lhjnilsson/foreverbull/internal/test_helper"
	"github.com/lhjnilsson/foreverbull/pkg/service/pb"
	"github.com/stretchr/testify/suite"
	"go.nanomsg.org/mangos/v3"
	"go.nanomsg.org/mangos/v3/protocol/rep"
	"go.nanomsg.org/mangos/v3/protocol/req"
	_ "go.nanomsg.org/mangos/v3/transport/all"
)

type PoolTest struct {
	suite.Suite
	pool *pool

	socket          mangos.Socket
	namespaceSocket mangos.Socket
}

func (test *PoolTest) SetupSuite() {
	test_helper.SetupEnvironment(test.T(), &test_helper.Containers{})
}

func TestPool(t *testing.T) {
	suite.Run(t, new(PoolTest))
}

func (test *PoolTest) TestSimple() {
	cb := func(req *pb.WorkerRequest) *pb.WorkerResponse {
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
	pool, err := NewPool(context.TODO(), algo)

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
