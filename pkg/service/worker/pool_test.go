package worker_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/internal/test_helper"
	finance_pb "github.com/lhjnilsson/foreverbull/pkg/pb/finance"
	servicePb "github.com/lhjnilsson/foreverbull/pkg/pb/service/service"
	"github.com/lhjnilsson/foreverbull/pkg/service/worker"
	"github.com/stretchr/testify/suite"
	"go.nanomsg.org/mangos/v3"
	"go.nanomsg.org/mangos/v3/protocol/rep"
	"go.nanomsg.org/mangos/v3/protocol/req"
	_ "go.nanomsg.org/mangos/v3/transport/all" // for mangos transport.
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type PoolTest struct {
	suite.Suite
}

func (test *PoolTest) SetupSuite() {
	test_helper.SetupEnv(test.T())
}

func TestPool(t *testing.T) {
	suite.Run(t, new(PoolTest))
}

func (test *PoolTest) TestSimple() {
	cb := func(_ *servicePb.WorkerRequest) *servicePb.WorkerResponse {
		return &servicePb.WorkerResponse{}
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
	workerURL, err := environment.WorkerDatabaseURL()
	test.Require().NoError(err)
	test.Equal(workerURL, configuration.GetDatabaseURL())
	test.NotEqual(environment.GetPostgresURL(), configuration.GetDatabaseURL())
	test.NotContains(configuration.GetDatabaseURL(), "postgres:foreverbull@")
	test.Contains(configuration.GetDatabaseURL(), environment.WorkerPostgresRole+":")

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
	test.Require().NoError(pool.Close())
}

func (test *PoolTest) startWorker(functions ...*test_helper.WorkerFunction) (worker.Pool, mangos.Socket) {
	test.T().Helper()

	algo, runner := test_helper.WorkerSimulator(test.T(), functions...)
	pool, err := worker.NewPool(context.TODO(), algo)
	test.Require().NoError(err)

	configuration := pool.Configure()
	socket, err := rep.NewSocket()
	test.Require().NoError(err)
	test.Require().NoError(socket.Dial(fmt.Sprintf("tcp://localhost:%d", configuration.BrokerPort)))
	test.Require().NoError(socket.SetOption(mangos.OptionRecvDeadline, 5*time.Second))
	test.Require().NoError(socket.SetOption(mangos.OptionSendDeadline, 5*time.Second))
	go runner(socket)

	return pool, socket
}

func (test *PoolTest) TestProcessWorkerRequestContract() {
	var mu sync.Mutex
	var requests []*servicePb.WorkerRequest
	handler := func(req *servicePb.WorkerRequest) *servicePb.WorkerResponse {
		mu.Lock()
		defer mu.Unlock()
		requests = append(requests, proto.Clone(req).(*servicePb.WorkerRequest))
		orders := make([]*finance_pb.Order, 0, len(req.Symbols))
		for _, symbol := range req.Symbols {
			orders = append(orders, &finance_pb.Order{Symbol: symbol, Amount: 4})
		}
		return &servicePb.WorkerResponse{Task: req.Task, Orders: orders}
	}

	pool, socket := test.startWorker(
		&test_helper.WorkerFunction{CB: handler, Name: "handle_data", Parallel: false},
	)
	defer socket.Close()
	defer pool.Close()

	when := time.Date(2024, time.March, 9, 15, 4, 5, 0, time.UTC)
	portfolio := &finance_pb.Portfolio{Timestamp: timestamppb.New(when), Cash: 1000}
	orders, err := pool.Process(context.TODO(), when, []string{"AAPL", "MSFT"}, portfolio)
	test.Require().NoError(err)
	test.Require().Len(orders, 2)
	test.Equal("AAPL", orders[0].GetSymbol())
	test.Equal(int32(4), orders[0].GetAmount())
	test.Equal("MSFT", orders[1].GetSymbol())
	test.Equal(int32(4), orders[1].GetAmount())
	test.Require().Len(requests, 1)
	test.Equal("handle_data", requests[0].Task)
	test.Equal([]string{"AAPL", "MSFT"}, requests[0].Symbols)
	test.InDelta(1000.0, requests[0].Portfolio.GetCash(), 1e-9)
	test.True(when.Equal(requests[0].Portfolio.GetTimestamp().AsTime()))
}

func (test *PoolTest) TestProcessRunsFunctionsInDefinitionOrder() {
	var mu sync.Mutex
	var tasks []string
	var parallelSymbols [][]string
	handler := func(req *servicePb.WorkerRequest) *servicePb.WorkerResponse {
		mu.Lock()
		defer mu.Unlock()
		tasks = append(tasks, req.Task)
		if req.Task == "measure_assets" {
			copied := append([]string{}, req.Symbols...)
			parallelSymbols = append(parallelSymbols, copied)
		}
		return &servicePb.WorkerResponse{Task: req.Task}
	}

	pool, socket := test.startWorker(
		&test_helper.WorkerFunction{CB: handler, Name: "measure_assets", Parallel: true},
		&test_helper.WorkerFunction{CB: handler, Name: "create_orders", Parallel: false, RunLast: true},
		&test_helper.WorkerFunction{CB: handler, Name: "filter_assets", Parallel: false, RunFirst: true},
	)
	defer socket.Close()
	defer pool.Close()

	_, err := pool.Process(context.TODO(), time.Now(), []string{"AAPL", "MSFT"}, &finance_pb.Portfolio{})
	test.Require().NoError(err)
	test.Require().Len(tasks, 4)
	test.Equal("filter_assets", tasks[0])
	test.Equal("create_orders", tasks[3])
	test.ElementsMatch([]string{"measure_assets", "measure_assets"}, tasks[1:3])
	test.ElementsMatch([][]string{{"AAPL"}, {"MSFT"}}, parallelSymbols)
}
