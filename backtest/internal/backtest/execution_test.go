package backtest

import (
	"context"
	"testing"

	"github.com/lhjnilsson/foreverbull/backtest/entity"
	"github.com/lhjnilsson/foreverbull/service/backtest"
	"github.com/lhjnilsson/foreverbull/service/worker"
	mockBacktest "github.com/lhjnilsson/foreverbull/tests/mocks/service/backtest"
	mockWorker "github.com/lhjnilsson/foreverbull/tests/mocks/service/worker"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type ExecutionTest struct {
	suite.Suite
	backtest  *mockBacktest.Backtest
	workers   *mockWorker.Pool
	execution *execution
}

func (test *ExecutionTest) SetupTest() {
	test.backtest = new(mockBacktest.Backtest)
	test.workers = new(mockWorker.Pool)
	test.execution = NewExecution(test.backtest, test.workers).(*execution)
}

func TestExecution(t *testing.T) {
	suite.Run(t, new(ExecutionTest))
}

func (test *ExecutionTest) TestConfigure() {
	workercfg := &worker.Configuration{}
	backtestcfg := &backtest.BacktestConfig{}

	test.backtest.On("ConfigureExecution", mock.Anything, backtestcfg).Return(nil)
	test.workers.On("ConfigureExecution", mock.Anything, workercfg).Return(nil)

	err := test.execution.Configure(context.TODO(), workercfg, backtestcfg)
	test.Nil(err)
}

func (test *ExecutionTest) TestRun() {
	test.backtest.On("RunExecution", mock.Anything).Return(nil)
	test.workers.On("RunExecution", mock.Anything).Return(nil)

	test.backtest.On("GetMessage").Return(nil, nil)

	test.NoError(test.execution.Run(context.Background(), &entity.Execution{}))
}

func (test *ExecutionTest) TestStop() {
	test.backtest.On("Stop", mock.Anything).Return(nil)
	test.workers.On("Stop", mock.Anything).Return(nil)

	err := test.execution.Stop(context.Background())
	test.Nil(err)
}
