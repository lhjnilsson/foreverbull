package worker

import (
	"context"
	"testing"

	"github.com/lhjnilsson/foreverbull/service/entity"
	"github.com/lhjnilsson/foreverbull/service/message"
	"github.com/lhjnilsson/foreverbull/service/socket"
	"github.com/lhjnilsson/foreverbull/tests/helper"
	mockSocket "github.com/lhjnilsson/foreverbull/tests/mocks/service/socket"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	_ "go.nanomsg.org/mangos/v3/transport/all"
)

type PoolTest struct {
	suite.Suite
	pool           *pool
	worker         *Instance
	instanceSocket *mockSocket.ContextSocket
	poolSocket     *mockSocket.ContextSocket
	rw             *mockSocket.ReadWriter
}

func (test *PoolTest) SetupTest() {
	test.instanceSocket = mockSocket.NewContextSocket(test.T())
	test.poolSocket = mockSocket.NewContextSocket(test.T())

	test.rw = mockSocket.NewReadWriter(test.T())
	test.worker = &Instance{
		socket: test.instanceSocket,
	}
	test.pool = &pool{
		Workers: make([]*Instance, 0),
		Socket:  &socket.Socket{},
		socket:  test.poolSocket,
	}
	test.pool.Workers = append(test.pool.Workers, test.worker)
	helper.SetupEnvironment(test.T(), &helper.Containers{})
}

func (test *PoolTest) TearDownTest() {

}

func TestPool(t *testing.T) {
	suite.Run(t, new(PoolTest))
}

func (test *PoolTest) TestConfigure() {
	rsp := message.Response{Task: "configure"}
	data, _ := rsp.Encode()

	test.rw.On("Close").Return(nil)
	test.instanceSocket.On("Get").Return(test.rw, nil)
	test.rw.On("Write", mock.Anything).Return(nil)
	test.rw.On("Read").Return(data, nil)
	err := test.pool.ConfigureExecution(context.Background(), &entity.Instance{})
	test.NoError(err)
}

func (test *PoolTest) TestConfigureError() {
	rsp := message.Response{Task: "configure", Error: "General Error"}
	data, _ := rsp.Encode()

	test.rw.On("Close").Return(nil)
	test.instanceSocket.On("Get").Return(test.rw, nil)
	test.rw.On("Write", mock.Anything).Return(nil)
	test.rw.On("Read").Return(data, nil)
	err := test.pool.ConfigureExecution(context.Background(), &entity.Instance{})
	test.Error(err)
	test.EqualError(err, "error configuring worker: error configuring instance: General Error")
}

func (test *PoolTest) TestConfigureErrorNoWorkers() {
	test.pool.Workers = make([]*Instance, 0)
	err := test.pool.ConfigureExecution(context.Background(), &entity.Instance{})
	test.Error(err)
	test.EqualError(err, "no workers")
}

func (test *PoolTest) TestRun() {
	rsp := message.Response{Task: "run"}
	data, _ := rsp.Encode()

	test.rw.On("Close").Return(nil)
	test.instanceSocket.On("Get").Return(test.rw, nil)
	test.rw.On("Write", mock.Anything).Return(nil)
	test.rw.On("Read").Return(data, nil)
	err := test.pool.RunExecution(context.Background())
	test.NoError(err)
}

func (test *PoolTest) TestRunError() {
	rsp := message.Response{Task: "run", Error: "General Error"}
	data, _ := rsp.Encode()

	test.rw.On("Close").Return(nil)
	test.instanceSocket.On("Get").Return(test.rw, nil)
	test.rw.On("Write", mock.Anything).Return(nil)
	test.rw.On("Read").Return(data, nil)
	err := test.pool.RunExecution(context.Background())
	test.Error(err)
	test.EqualError(err, "error running worker: error running instance: General Error")
}

/*
func (test *PoolTest) TestProcess() {
	rsp := message.Response{Task: "process"}
	data, _ := rsp.Encode()

	socket := mockSocket.NewContextSocket(test.T())
	rw := mockSocket.NewReadWriter(test.T())

	rw.On("Close").Return(nil)
	socket.On("Get").Return(rw, nil)
	rw.On("Write", mock.Anything).Return(nil)
	rw.On("Read").Return(data, nil)

	test.pool.socket = socket

	_, err := test.pool.Process(context.Background(), &message.Request{})
	test.NoError(err)
}

func (test *PoolTest) TestProcessError() {
	rsp := message.Response{Task: "process", Error: "General Error"}
	data, _ := rsp.Encode()

	socket := mockSocket.NewContextSocket(test.T())
	rw := mockSocket.NewReadWriter(test.T())

	rw.On("Close").Return(nil)
	socket.On("Get").Return(rw, nil)
	rw.On("Write", mock.Anything).Return(nil)
	rw.On("Read").Return(data, nil)

	test.pool.socket = socket
	_, err := test.pool.Process(context.Background(), &message.Request{})
	test.Error(err)
	test.EqualError(err, "error processing request: General Error")
}
*/

func (test *PoolTest) TestStop() {
	rsp := message.Response{Task: "stop"}
	data, _ := rsp.Encode()

	test.rw.On("Close").Return(nil)
	test.instanceSocket.On("Get").Return(test.rw, nil)
	test.poolSocket.On("Close").Return(nil)
	test.rw.On("Write", mock.Anything).Return(nil)
	test.rw.On("Read").Return(data, nil)
	err := test.pool.Stop(context.Background())
	test.NoError(err)
}

func (test *PoolTest) TestStopError() {
	rsp := message.Response{Task: "stop", Error: "General Error"}
	data, _ := rsp.Encode()

	test.rw.On("Close").Return(nil)
	test.instanceSocket.On("Get").Return(test.rw, nil)
	test.poolSocket.On("Close").Return(nil)
	test.rw.On("Write", mock.Anything).Return(nil)
	test.rw.On("Read").Return(data, nil)
	err := test.pool.Stop(context.Background())
	test.Error(err)
	test.EqualError(err, "error stopping worker: General Error")
}
