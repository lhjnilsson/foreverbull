package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"testing"
	"time"

	service_pb "github.com/lhjnilsson/foreverbull/internal/pb/service"
	"google.golang.org/protobuf/proto"

	"github.com/lhjnilsson/foreverbull/internal/test_helper"
	finance "github.com/lhjnilsson/foreverbull/pkg/finance/entity"
	"github.com/lhjnilsson/foreverbull/pkg/service/entity"
	"github.com/shopspring/decimal"
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

func (test *PoolTest) SetupTest() {
	p, err := NewPool(context.TODO(), nil)
	test.Require().NoError(err)
	pool, t := p.(*pool)
	test.Require().True(t)
	test.pool = pool

	test.socket, err = rep.NewSocket()
	test.Require().NoError(err)
	test.Require().NoError(test.socket.Dial(fmt.Sprintf("tcp://localhost:%d", test.pool.GetPort())))
	test.Require().NoError(test.socket.SetOption(mangos.OptionRecvDeadline, time.Second))
	test.Require().NoError(test.socket.SetOption(mangos.OptionSendDeadline, time.Second))
	test.namespaceSocket, err = req.NewSocket()
	test.Require().NoError(err)
	err = test.namespaceSocket.Dial(fmt.Sprintf("tcp://localhost:%d", test.pool.GetNamespacePort()))
	test.Require().NoError(test.namespaceSocket.SetOption(mangos.OptionRecvDeadline, time.Second))
	test.Require().NoError(test.namespaceSocket.SetOption(mangos.OptionSendDeadline, time.Second))
	test.Require().NoError(err)
}

func (test *PoolTest) TearDownTest() {
	if test.pool != nil {
		test.NoError(test.pool.Close())
	}
	if test.socket != nil {
		test.NoError(test.socket.Close())
	}
	if test.namespaceSocket != nil {
		test.NoError(test.namespaceSocket.Close())
	}
}

func TestPool(t *testing.T) {
	suite.Run(t, new(PoolTest))
}

func (test *PoolTest) TestNamespaceSocket() {
	algoS := `{
		"functions": [],
		"namespace": {
			"rsi": {
				"type": "object",
				"value_type": "float64"
			}
		}
	}`
	algo := &entity.Algorithm{}
	err := json.Unmarshal([]byte(algoS), algo)
	test.Require().NoError(err)

	test.NoError(test.pool.SetAlgorithm(algo))
}

func (test *PoolTest) TestProcessNonParallel() {
	algo := &entity.Algorithm{
		Functions: []entity.AlgorithmFunction{
			{
				Name:              "handle_data",
				ParallelExecution: false,
			},
		},
	}
	test.NoError(test.pool.SetAlgorithm(algo))

	cash, err := decimal.NewFromString("100000")
	test.Require().NoError(err)
	value, err := decimal.NewFromString("1000")
	test.Require().NoError(err)
	symbols := []string{"aaple", "tsla", "msft"}
	timestamp := time.Now().UTC()
	portfolio := &finance.Portfolio{
		Cash:      cash,
		Value:     value,
		Positions: []finance.Position{},
	}

	go func() {
		_, err := test.pool.Process(context.TODO(), timestamp, symbols, portfolio)
		test.Require().NoError(err)
	}()

	bytes, err := test.socket.Recv()
	test.Require().NoError(err)
	request := &service_pb.WorkerRequest{}
	test.NoError(proto.Unmarshal(bytes, request))

	test.Equal("handle_data", request.Task)
	test.Equal(timestamp, request.Timestamp.AsTime())
	test.Equal(symbols, request.Symbols)
	test.Require().NotNil(request.Portfolio)
	test.Equal(portfolio.Cash.InexactFloat64(), request.Portfolio.Cash)
	test.Equal(portfolio.Value.InexactFloat64(), request.Portfolio.Value)
	//test.Equal(portfolio.Positions, request.Portfolio.Positions)

	response := &service_pb.WorkerResponse{
		Task: request.Task,
	}
	bytes, err = proto.Marshal(response)
	test.Require().NoError(err)
	test.Require().NoError(test.socket.Send(bytes))
	time.Sleep(time.Second)
}

func (test *PoolTest) TestProcessParallel() {
	algo := &entity.Algorithm{
		Functions: []entity.AlgorithmFunction{
			{
				Name:              "handle_data",
				ParallelExecution: true,
			},
		},
	}
	test.NoError(test.pool.SetAlgorithm(algo))

	cash, err := decimal.NewFromString("100000")
	test.Require().NoError(err)
	value, err := decimal.NewFromString("1000")
	test.Require().NoError(err)
	symbols := []string{"aaple", "msft", "tsla"}
	timestamp := time.Now().UTC()
	portfolio := &finance.Portfolio{
		Cash:      cash,
		Value:     value,
		Positions: []finance.Position{},
	}

	go func() {
		_, err := test.pool.Process(context.TODO(), timestamp, symbols, portfolio)
		test.Require().NoError(err)
	}()

	recievedSymbols := []string{}
	for _, _ = range symbols {
		bytes, err := test.socket.Recv()
		test.Require().NoError(err)
		request := &service_pb.WorkerRequest{}
		test.NoError(proto.Unmarshal(bytes, request))

		test.Equal("handle_data", request.Task)
		test.Equal(timestamp, request.Timestamp.AsTime())
		test.Equal(portfolio.Cash.InexactFloat64(), request.Portfolio.Cash)
		test.Equal(portfolio.Value.InexactFloat64(), request.Portfolio.Value)
		//test.Equal(portfolio.Positions, request.Portfolio.Positions)

		response := &service_pb.WorkerResponse{
			Task: request.Task,
		}
		bytes, err = proto.Marshal(response)
		test.Require().NoError(err)
		test.Require().NoError(test.socket.Send(bytes))

		recievedSymbols = append(recievedSymbols, request.Symbols...)
	}
	slices.Sort(recievedSymbols)
	test.EqualValues(symbols, recievedSymbols)
	time.Sleep(time.Second)
}
