package worker

import (
	"context"
	"testing"

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

func (s *PoolTest) SetupTest() {
	s.instanceSocket = mockSocket.NewContextSocket(s.T())
	s.poolSocket = mockSocket.NewContextSocket(s.T())

	s.rw = mockSocket.NewReadWriter(s.T())
	s.worker = &Instance{
		socket: s.instanceSocket,
	}
	s.pool = &pool{
		Workers: make([]*Instance, 0),
		Socket:  &socket.Socket{},
		socket:  s.poolSocket,
	}
	s.pool.Workers = append(s.pool.Workers, s.worker)
	helper.SetupEnvironment(s.T(), &helper.Containers{})
}

func (s *PoolTest) TearDownTest() {

}

func TestPool(t *testing.T) {
	suite.Run(t, new(PoolTest))
}

func (s *PoolTest) TestConfigure() {
	rsp := message.Response{Task: "configure"}
	data, _ := rsp.Encode()

	s.rw.On("Close").Return(nil)
	s.instanceSocket.On("Get").Return(s.rw, nil)
	s.rw.On("Write", mock.Anything).Return(nil)
	s.rw.On("Read").Return(data, nil)
	err := s.pool.ConfigureExecution(context.Background(), &Configuration{})
	s.NoError(err)
}

func (s *PoolTest) TestConfigureError() {
	rsp := message.Response{Task: "configure", Error: "General Error"}
	data, _ := rsp.Encode()

	s.rw.On("Close").Return(nil)
	s.instanceSocket.On("Get").Return(s.rw, nil)
	s.rw.On("Write", mock.Anything).Return(nil)
	s.rw.On("Read").Return(data, nil)
	err := s.pool.ConfigureExecution(context.Background(), &Configuration{})
	s.Error(err)
	s.EqualError(err, "error configuring worker: error configuring instance: General Error")
}

func (s *PoolTest) TestConfigureErrorNoWorkers() {
	s.pool.Workers = make([]*Instance, 0)
	err := s.pool.ConfigureExecution(context.Background(), &Configuration{})
	s.Error(err)
	s.EqualError(err, "no workers")
}

func (s *PoolTest) TestRun() {
	rsp := message.Response{Task: "run"}
	data, _ := rsp.Encode()

	s.rw.On("Close").Return(nil)
	s.instanceSocket.On("Get").Return(s.rw, nil)
	s.rw.On("Write", mock.Anything).Return(nil)
	s.rw.On("Read").Return(data, nil)
	err := s.pool.RunExecution(context.Background())
	s.NoError(err)
}

func (s *PoolTest) TestRunError() {
	rsp := message.Response{Task: "run", Error: "General Error"}
	data, _ := rsp.Encode()

	s.rw.On("Close").Return(nil)
	s.instanceSocket.On("Get").Return(s.rw, nil)
	s.rw.On("Write", mock.Anything).Return(nil)
	s.rw.On("Read").Return(data, nil)
	err := s.pool.RunExecution(context.Background())
	s.Error(err)
	s.EqualError(err, "error running worker: error running instance: General Error")
}

/*
func (s *PoolTest) TestProcess() {
	rsp := message.Response{Task: "process"}
	data, _ := rsp.Encode()

	socket := mockSocket.NewContextSocket(s.T())
	rw := mockSocket.NewReadWriter(s.T())

	rw.On("Close").Return(nil)
	socket.On("Get").Return(rw, nil)
	rw.On("Write", mock.Anything).Return(nil)
	rw.On("Read").Return(data, nil)

	s.pool.socket = socket

	_, err := s.pool.Process(context.Background(), &message.Request{})
	s.NoError(err)
}

func (s *PoolTest) TestProcessError() {
	rsp := message.Response{Task: "process", Error: "General Error"}
	data, _ := rsp.Encode()

	socket := mockSocket.NewContextSocket(s.T())
	rw := mockSocket.NewReadWriter(s.T())

	rw.On("Close").Return(nil)
	socket.On("Get").Return(rw, nil)
	rw.On("Write", mock.Anything).Return(nil)
	rw.On("Read").Return(data, nil)

	s.pool.socket = socket
	_, err := s.pool.Process(context.Background(), &message.Request{})
	s.Error(err)
	s.EqualError(err, "error processing request: General Error")
}
*/

func (s *PoolTest) TestStop() {
	rsp := message.Response{Task: "stop"}
	data, _ := rsp.Encode()

	s.rw.On("Close").Return(nil)
	s.instanceSocket.On("Get").Return(s.rw, nil)
	s.poolSocket.On("Close").Return(nil)
	s.rw.On("Write", mock.Anything).Return(nil)
	s.rw.On("Read").Return(data, nil)
	err := s.pool.Stop(context.Background())
	s.NoError(err)
}

func (s *PoolTest) TestStopError() {
	rsp := message.Response{Task: "stop", Error: "General Error"}
	data, _ := rsp.Encode()

	s.rw.On("Close").Return(nil)
	s.instanceSocket.On("Get").Return(s.rw, nil)
	s.poolSocket.On("Close").Return(nil)
	s.rw.On("Write", mock.Anything).Return(nil)
	s.rw.On("Read").Return(data, nil)
	err := s.pool.Stop(context.Background())
	s.Error(err)
	s.EqualError(err, "error stopping worker: General Error")
}
