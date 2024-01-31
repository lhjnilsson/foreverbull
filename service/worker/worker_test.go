package worker

import (
	"context"
	"errors"
	"testing"

	"github.com/lhjnilsson/foreverbull/service/message"
	mockSocket "github.com/lhjnilsson/foreverbull/tests/mocks/service/socket"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type WorkerTest struct {
	suite.Suite
	socket *mockSocket.ContextSocket
	rw     *mockSocket.ReadWriter
	worker *Instance
}

func (s *WorkerTest) SetupTest() {
	s.socket = mockSocket.NewContextSocket(s.T())
	s.worker = &Instance{
		socket: s.socket,
	}
	s.rw = mockSocket.NewReadWriter(s.T())
	s.socket.On("Get").Return(s.rw, nil)
	s.rw.On("Close").Return(nil)
}

func (s *WorkerTest) TearDownTest() {
}

func TestWorker(t *testing.T) {
	suite.Run(t, new(WorkerTest))
}

func (s *WorkerTest) TestConfigureNormal() {
	rsp := message.Response{Task: "configure"}
	data, _ := rsp.Encode()

	s.rw.On("Write", mock.Anything).Return(nil)
	s.rw.On("Read").Return(data, nil)
	err := s.worker.ConfigureExecution(context.Background(), &Configuration{})
	s.NoError(err)
}

func (s *WorkerTest) TestConfigureError() {
	rsp := message.Response{Task: "configure", Error: "General Error"}
	data, _ := rsp.Encode()

	s.rw.On("Write", mock.Anything).Return(nil)
	s.rw.On("Read").Return(data, nil)
	err := s.worker.ConfigureExecution(context.Background(), &Configuration{})
	s.Error(err)
	s.EqualError(err, "error configuring instance: General Error")
}

func (s *WorkerTest) TestConfigureSocketError() {
	s.rw.On("Write", mock.Anything).Return(errors.New("Write Error"))
	err := s.worker.ConfigureExecution(context.Background(), &Configuration{})
	s.Error(err)
	s.EqualError(err, "error configuring instance: Write Error")
}

func (s *WorkerTest) TestRunNormal() {
	rsp := message.Response{Task: "run_backtest"}
	data, _ := rsp.Encode()

	s.rw.On("Write", mock.Anything).Return(nil)
	s.rw.On("Read").Return(data, nil)
	err := s.worker.RunExecution(context.Background())
	s.NoError(err)
}

func (s *WorkerTest) TestRunError() {
	rsp := message.Response{Task: "run_backtest", Error: "General Error"}
	data, _ := rsp.Encode()

	s.rw.On("Write", mock.Anything).Return(nil)
	s.rw.On("Read").Return(data, nil)
	err := s.worker.RunExecution(context.Background())
	s.Error(err)
	s.EqualError(err, "error running instance: General Error")
}

func (s *WorkerTest) TestRunSocketError() {
	s.rw.On("Write", mock.Anything).Return(errors.New("Write Error"))
	err := s.worker.RunExecution(context.Background())
	s.Error(err)
	s.EqualError(err, "error running instance: Write Error")
}

func (s *WorkerTest) TestStopNormal() {
	rsp := message.Response{Task: "stop"}
	data, _ := rsp.Encode()

	s.rw.On("Write", mock.Anything).Return(nil)
	s.rw.On("Read").Return(data, nil)
	err := s.worker.Stop(context.Background())
	s.NoError(err)
}

func (s *WorkerTest) TestStopError() {
	rsp := message.Response{Task: "stop", Error: "General Error"}
	data, _ := rsp.Encode()

	s.rw.On("Write", mock.Anything).Return(nil)
	s.rw.On("Read").Return(data, nil)
	err := s.worker.Stop(context.Background())
	s.Error(err)
	s.EqualError(err, "General Error")
}

func (s *WorkerTest) TestStopSocketError() {
	s.rw.On("Write", mock.Anything).Return(errors.New("Write Error"))
	err := s.worker.Stop(context.Background())
	s.Error(err)
	s.EqualError(err, "Write Error")
}
