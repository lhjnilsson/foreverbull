package backtest

import (
	"context"
	"testing"

	"github.com/lhjnilsson/foreverbull/pkg/backtest/engine"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/entity"
	service "github.com/lhjnilsson/foreverbull/pkg/service/entity"
	"github.com/lhjnilsson/foreverbull/pkg/service/worker"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type ExecutionTest struct {
	suite.Suite
	backtest  *engine.MockEngine
	workers   *worker.MockPool
	execution *execution
}

func (test *ExecutionTest) SetupTest() {
	test.backtest = new(engine.MockEngine)
	test.workers = new(worker.MockPool)
	test.execution = NewExecution(test.backtest, test.workers).(*execution)
}

func TestExecution(t *testing.T) {
	suite.Run(t, new(ExecutionTest))
}

func (test *ExecutionTest) TestConfigure() {
	workercfg := &service.Instance{}
	execution := &entity.Execution{}

	test.backtest.On("ConfigureExecution", mock.Anything, execution).Return(nil)
	test.workers.On("ConfigureExecution", mock.Anything, workercfg).Return(nil)

	err := test.execution.Configure(context.TODO(), execution)
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
