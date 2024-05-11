package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"testing"
	"time"

	finance "github.com/lhjnilsson/foreverbull/finance/entity"
	"github.com/lhjnilsson/foreverbull/service/entity"
	"github.com/lhjnilsson/foreverbull/service/message"
	"github.com/lhjnilsson/foreverbull/tests/helper"
	"github.com/mitchellh/mapstructure"
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
	helper.SetupEnvironment(test.T(), &helper.Containers{})
}

func (test *PoolTest) SetupTest() {
	p, err := NewPool(context.TODO(), nil)
	test.Require().NoError(err)
	pool, t := p.(*pool)
	test.Require().True(t)
	test.pool = pool

	test.socket, err = rep.NewSocket()
	test.Require().NoError(err)
	err = test.socket.Dial(fmt.Sprintf("tcp://localhost:%d", test.pool.GetPort()))
	test.Require().NoError(test.socket.SetOption(mangos.OptionRecvDeadline, time.Second))
	test.Require().NoError(test.socket.SetOption(mangos.OptionSendDeadline, time.Second))
	test.Require().NoError(err)
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

	for _, data := range []string{`{ "aaple": 14.0 }`, `{ "tsla": 15.0 }`, `{ "msft": 16.0 }`} {
		request := message.Request{
			Task: "set:rsi",
			Data: data,
		}
		bytes, err := json.Marshal(request)
		test.Require().NoError(err)
		test.Require().NoError(test.namespaceSocket.Send(bytes))

		bytes, err = test.namespaceSocket.Recv()
		test.Require().NoError(err)
		response := message.Response{}
		test.Require().NoError(json.Unmarshal(bytes, &response))
		test.Empty(response.Error)
	}

	request := message.Request{
		Task: "get:rsi",
	}
	bytes, err := json.Marshal(request)
	test.Require().NoError(err)
	test.Require().NoError(test.namespaceSocket.Send(bytes))

	bytes, err = test.namespaceSocket.Recv()
	test.Require().NoError(err)
	response := message.Response{}
	test.Require().NoError(json.Unmarshal(bytes, &response))
	test.Empty(response.Error)
	test.NotEmpty(response.Data)
	data := map[string]float64{}
	err = mapstructure.Decode(response.Data, &data)
	test.Require().NoError(err)
	test.Equal(3, len(data))
	test.Equal(14.0, data["aaple"])
	test.Equal(15.0, data["tsla"])
	test.Equal(16.0, data["msft"])
}

func (test *PoolTest) TestProcessNonParallel() {
	algoS := `{
		"functions": [
			{
				"name": "handle_data",
				"parameters": [],
				"parallel_execution": false
			}
		]
	}`
	algo := &entity.Algorithm{}
	err := json.Unmarshal([]byte(algoS), algo)
	test.Require().NoError(err)

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

	go test.pool.Process(context.TODO(), timestamp, symbols, portfolio)

	bytes, err := test.socket.Recv()
	test.Require().NoError(err)
	request := &message.Request{}
	test.Require().NoError(request.Decode(bytes))

	test.Equal("handle_data", request.Task)
	test.NotEmpty(request.Data)

	wReq := Request{}
	err = request.DecodeData(&wReq)
	test.Require().NoError(err)
	test.Equal(timestamp, wReq.Timestamp)
	test.Equal(symbols, wReq.Symbols)
	test.Equal(portfolio.Cash, wReq.Portfolio.Cash)
	test.Equal(portfolio.Value, wReq.Portfolio.Value)
	test.Equal(portfolio.Positions, wReq.Portfolio.Positions)

	response := &message.Response{
		Task: request.Task,
	}
	bytes, err = response.Encode()
	test.Require().NoError(err)
	test.Require().NoError(test.socket.Send(bytes))
}

func (test *PoolTest) TestProcessParallel() {
	algoS := `{
		"functions": [
			{
				"name": "handle_data",
				"parameters": [],
				"parallel_execution": true
			}
		]
	}`
	algo := &entity.Algorithm{}
	err := json.Unmarshal([]byte(algoS), algo)
	test.Require().NoError(err)

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

	go test.pool.Process(context.TODO(), timestamp, symbols, portfolio)

	recievedSymbols := []string{}
	for _, _ = range symbols {
		bytes, err := test.socket.Recv()
		test.Require().NoError(err)
		request := &message.Request{}
		test.Require().NoError(request.Decode(bytes))

		test.Equal("handle_data", request.Task)
		test.NotEmpty(request.Data)

		wReq := Request{}
		err = request.DecodeData(&wReq)
		test.Require().NoError(err)
		test.Equal(timestamp, wReq.Timestamp)
		test.Equal(portfolio.Cash, wReq.Portfolio.Cash)
		test.Equal(portfolio.Value, wReq.Portfolio.Value)
		test.Equal(portfolio.Positions, wReq.Portfolio.Positions)

		response := &message.Response{
			Task: request.Task,
		}
		bytes, err = response.Encode()
		test.Require().NoError(err)
		test.Require().NoError(test.socket.Send(bytes))

		recievedSymbols = append(recievedSymbols, wReq.Symbols...)
	}
	slices.Sort(recievedSymbols)
	test.EqualValues(symbols, recievedSymbols)
}
