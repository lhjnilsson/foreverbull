package worker_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

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

func (test *PoolTest) TestConfigureDoesNotSendADatabaseURL() {
	algo, _ := test_helper.WorkerSimulator(test.T(), &test_helper.WorkerFunction{Name: "test"})
	pool, err := worker.NewPool(context.TODO(), algo)
	test.Require().NoError(err)
	defer pool.Close()

	configuration := pool.Configure()
	test.Empty(configuration.GetDatabaseURL())
	test.NotContains(configuration.String(), "postgres://")
	test.NotContains(configuration.String(), "foreverbull@")
}

type recordingBars struct {
	calls    int
	sizes    []int
	provider string
	asOf     time.Time
	limit    int
	bars     []*finance_pb.OHLC
	err      error
}

func (r *recordingBars) List(_ context.Context, provider string, symbols []string, asOf time.Time, limit int) ([]*finance_pb.OHLC, error) {
	r.calls++
	r.sizes = append(r.sizes, len(symbols))
	r.provider = provider
	r.asOf = asOf
	r.limit = limit
	if r.err != nil {
		return nil, r.err
	}
	return r.bars, nil
}

func (test *PoolTest) TestProcessSendsBarsWithoutADatabaseURL() {
	when := time.Date(2024, time.March, 9, 15, 4, 5, 0, time.UTC)
	source := &recordingBars{bars: []*finance_pb.OHLC{
		&finance_pb.OHLC{Symbol: "AAPL", Close: 10, Timestamp: timestamppb.New(when)},
		&finance_pb.OHLC{Symbol: "MSFT", Close: 11, Timestamp: timestamppb.New(when)},
	}}

	var mu sync.Mutex
	var requests []*servicePb.WorkerRequest
	handler := func(req *servicePb.WorkerRequest) *servicePb.WorkerResponse {
		mu.Lock()
		defer mu.Unlock()
		requests = append(requests, proto.Clone(req).(*servicePb.WorkerRequest))
		return &servicePb.WorkerResponse{}
	}
	algo, runner := test_helper.WorkerSimulator(test.T(), &test_helper.WorkerFunction{
		CB: handler, Name: "handle_data", Parallel: true,
	})
	pool, err := worker.NewPool(context.TODO(), algo, worker.WithBars(source, "stooq"))
	test.Require().NoError(err)
	defer pool.Close()

	configuration := pool.Configure()
	test.Empty(configuration.GetDatabaseURL())
	configBytes, err := proto.Marshal(configuration)
	test.Require().NoError(err)
	test.NotContains(string(configBytes), "postgres://")

	socket, err := rep.NewSocket()
	test.Require().NoError(err)
	defer socket.Close()
	test.Require().NoError(socket.Dial(fmt.Sprintf("tcp://localhost:%d", configuration.BrokerPort)))
	test.Require().NoError(socket.SetOption(mangos.OptionRecvDeadline, time.Second))
	test.Require().NoError(socket.SetOption(mangos.OptionSendDeadline, time.Second))
	go runner(socket)

	portfolio := &finance_pb.Portfolio{Timestamp: timestamppb.New(when)}
	_, err = pool.Process(context.TODO(), when.Add(time.Hour), []string{"AAPL", "MSFT"}, portfolio)
	test.Require().NoError(err)
	test.Equal("stooq", source.provider)
	test.Equal(252, source.limit)
	test.True(when.Equal(source.asOf))
	test.Require().Len(requests, 2)
	bySymbol := map[string]string{}
	for _, req := range requests {
		test.Require().Len(req.GetBars(), 1)
		bySymbol[req.GetSymbols()[0]] = req.GetBars()[0].GetSymbol()
	}
	test.Equal("AAPL", bySymbol["AAPL"])
	test.Equal("MSFT", bySymbol["MSFT"])
	raw, err := proto.Marshal(requests[0])
	test.Require().NoError(err)
	test.NotContains(string(raw), "postgres://")
	test.NotContains(string(raw), "foreverbull@")
}

func (test *PoolTest) TestProcessChunksBarReads() {
	source := &recordingBars{}
	handler := func(_ *servicePb.WorkerRequest) *servicePb.WorkerResponse {
		return &servicePb.WorkerResponse{}
	}
	algo, runner := test_helper.WorkerSimulator(test.T(), &test_helper.WorkerFunction{CB: handler, Name: "handle_data"})
	pool, err := worker.NewPool(context.TODO(), algo, worker.WithBars(source, "yahoo"))
	test.Require().NoError(err)
	defer pool.Close()

	configuration := pool.Configure()
	socket, err := rep.NewSocket()
	test.Require().NoError(err)
	defer socket.Close()
	test.Require().NoError(socket.Dial(fmt.Sprintf("tcp://localhost:%d", configuration.BrokerPort)))
	test.Require().NoError(socket.SetOption(mangos.OptionRecvDeadline, time.Second))
	test.Require().NoError(socket.SetOption(mangos.OptionSendDeadline, time.Second))
	go runner(socket)

	symbols := make([]string, 501)
	for i := range symbols {
		symbols[i] = fmt.Sprintf("S%d", i)
	}
	_, err = pool.Process(context.TODO(), time.Now(), symbols, &finance_pb.Portfolio{})
	test.Require().NoError(err)
	test.Equal(2, source.calls)
	test.Equal([]int{500, 1}, source.sizes)
}

func (test *PoolTest) TestProcessReturnsABarLoadErrorBeforeSending() {
	source := &recordingBars{err: fmt.Errorf("read failed")}
	algo, _ := test_helper.WorkerSimulator(test.T(), &test_helper.WorkerFunction{Name: "handle_data"})
	pool, err := worker.NewPool(context.TODO(), algo, worker.WithBars(source, "yahoo"))
	test.Require().NoError(err)
	defer pool.Close()

	_, err = pool.Process(context.TODO(), time.Now(), []string{"AAPL"}, &finance_pb.Portfolio{})
	test.Require().EqualError(err, "load bars: read failed")
	test.NotContains(err.Error(), "postgres://")
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

func (test *PoolTest) TestProcessReturnsTheWorkerError() {
	message := "Couldn't run backtest: the data service can't read market data. This is a bug in the Foreverbull install, not in your strategy."
	cb := func(_ *servicePb.WorkerRequest) *servicePb.WorkerResponse {
		return &servicePb.WorkerResponse{Error: &message}
	}
	pool, socket := test.startWorker(&test_helper.WorkerFunction{CB: cb, Name: "handle_data"})
	defer socket.Close()
	defer pool.Close()

	_, err := pool.Process(context.TODO(), time.Now(), []string{"AAPL"}, &finance_pb.Portfolio{})
	test.Require().EqualError(err, message)
}

func (test *PoolTest) TestProcessReturnsTheWorkerErrorFromAParallelFunction() {
	message := "Couldn't run backtest: the data service can't read market data. This is a bug in the Foreverbull install, not in your strategy."
	cb := func(_ *servicePb.WorkerRequest) *servicePb.WorkerResponse {
		return &servicePb.WorkerResponse{Error: &message}
	}
	pool, socket := test.startWorker(&test_helper.WorkerFunction{CB: cb, Name: "handle_data", Parallel: true})
	defer socket.Close()
	defer pool.Close()

	_, err := pool.Process(context.TODO(), time.Now(), []string{"AAPL", "MSFT"}, &finance_pb.Portfolio{})
	test.Require().EqualError(err, message)
}
