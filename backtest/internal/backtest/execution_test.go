package backtest

import (
	"context"
	"testing"

	"github.com/lhjnilsson/foreverbull/backtest/entity"
	"github.com/lhjnilsson/foreverbull/service/backtest/engine"
	"github.com/lhjnilsson/foreverbull/service/message"
	"github.com/lhjnilsson/foreverbull/service/worker"
	mockEngine "github.com/lhjnilsson/foreverbull/tests/mocks/service/backtest/engine"
	mockWorker "github.com/lhjnilsson/foreverbull/tests/mocks/service/worker"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type ExecutionTest struct {
	suite.Suite
	engine    *mockEngine.Engine
	workers   *mockWorker.Pool
	execution *execution
}

func (test *ExecutionTest) SetupTest() {
	test.engine = new(mockEngine.Engine)
	test.workers = new(mockWorker.Pool)
	test.execution = NewExecution(test.engine, test.workers).(*execution)
}

func TestExecution(t *testing.T) {
	suite.Run(t, new(ExecutionTest))
}

func (test *ExecutionTest) TestConfigure() {
	workercfg := &worker.Configuration{}
	backtestcfg := &engine.BacktestConfig{}

	test.engine.On("ConfigureExecution", mock.Anything, backtestcfg).Return(nil)
	test.workers.On("ConfigureExecution", mock.Anything, workercfg).Return(nil)

	err := test.execution.Configure(context.TODO(), workercfg, backtestcfg)
	test.Nil(err)
}

func (test *ExecutionTest) TestRun() {
	test.engine.On("RunExecution", mock.Anything).Return(nil)
	test.workers.On("RunExecution", mock.Anything).Return(nil)

	test.engine.On("GetMessage").Return(&message.Response{Task: "period", Data: nil}, nil)

	events := make(chan chan entity.ExecutionPeriod)
	test.execution.Run(context.Background(), "test", events)
	test.Nil(<-events)
}

func (test *ExecutionTest) TestStop() {
	test.engine.On("Stop", mock.Anything).Return(nil)
	test.workers.On("Stop", mock.Anything).Return(nil)

	err := test.execution.Stop(context.Background())
	test.Nil(err)
}
