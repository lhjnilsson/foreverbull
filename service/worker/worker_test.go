package worker

import (
	"context"
	"errors"
	"testing"

	"github.com/lhjnilsson/foreverbull/service/entity"
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

func (test *WorkerTest) SetupTest() {
	test.socket = mockSocket.NewContextSocket(test.T())
	test.worker = &Instance{
		socket: test.socket,
	}
	test.rw = mockSocket.NewReadWriter(test.T())
	test.socket.On("Get").Return(test.rw, nil)
	test.rw.On("Close").Return(nil)
}

func (test *WorkerTest) TearDownTest() {
}

func TestWorker(t *testing.T) {
	suite.Run(t, new(WorkerTest))
}

func (test *WorkerTest) TestConfigureNormal() {
	rsp := message.Response{Task: "configure"}
	data, _ := rsp.Encode()

	test.rw.On("Write", mock.Anything).Return(nil)
	test.rw.On("Read").Return(data, nil)
	err := test.worker.ConfigureExecution(context.Background(), &entity.Instance{})
	test.NoError(err)
}

func (test *WorkerTest) TestConfigureError() {
	rsp := message.Response{Task: "configure", Error: "General Error"}
	data, _ := rsp.Encode()

	test.rw.On("Write", mock.Anything).Return(nil)
	test.rw.On("Read").Return(data, nil)
	err := test.worker.ConfigureExecution(context.Background(), &entity.Instance{})
	test.Error(err)
	test.EqualError(err, "error configuring instance: General Error")
}

func (test *WorkerTest) TestConfigureSocketError() {
	test.rw.On("Write", mock.Anything).Return(errors.New("Write Error"))
	err := test.worker.ConfigureExecution(context.Background(), &entity.Instance{})
	test.Error(err)
	test.EqualError(err, "error configuring instance: Write Error")
}

func (test *WorkerTest) TestRunNormal() {
	rsp := message.Response{Task: "run_backtest"}
	data, _ := rsp.Encode()

	test.rw.On("Write", mock.Anything).Return(nil)
	test.rw.On("Read").Return(data, nil)
	err := test.worker.RunExecution(context.Background())
	test.NoError(err)
}

func (test *WorkerTest) TestRunError() {
	rsp := message.Response{Task: "run_backtest", Error: "General Error"}
	data, _ := rsp.Encode()

	test.rw.On("Write", mock.Anything).Return(nil)
	test.rw.On("Read").Return(data, nil)
	err := test.worker.RunExecution(context.Background())
	test.Error(err)
	test.EqualError(err, "error running instance: General Error")
}

func (test *WorkerTest) TestRunSocketError() {
	test.rw.On("Write", mock.Anything).Return(errors.New("Write Error"))
	err := test.worker.RunExecution(context.Background())
	test.Error(err)
	test.EqualError(err, "error running instance: Write Error")
}

func (test *WorkerTest) TestStopNormal() {
	rsp := message.Response{Task: "stop"}
	data, _ := rsp.Encode()

	test.rw.On("Write", mock.Anything).Return(nil)
	test.rw.On("Read").Return(data, nil)
	err := test.worker.Stop(context.Background())
	test.NoError(err)
}

func (test *WorkerTest) TestStopError() {
	rsp := message.Response{Task: "stop", Error: "General Error"}
	data, _ := rsp.Encode()

	test.rw.On("Write", mock.Anything).Return(nil)
	test.rw.On("Read").Return(data, nil)
	err := test.worker.Stop(context.Background())
	test.Error(err)
	test.EqualError(err, "General Error")
}

func (test *WorkerTest) TestStopSocketError() {
	test.rw.On("Write", mock.Anything).Return(errors.New("Write Error"))
	err := test.worker.Stop(context.Background())
	test.Error(err)
	test.EqualError(err, "Write Error")
}
