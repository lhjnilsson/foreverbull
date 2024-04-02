package command

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	finance "github.com/lhjnilsson/foreverbull/finance/entity"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/internal/stream"
	"github.com/lhjnilsson/foreverbull/strategy/entity"
	"github.com/lhjnilsson/foreverbull/strategy/internal/repository"
	"github.com/lhjnilsson/foreverbull/strategy/internal/stream/dependency"
	ss "github.com/lhjnilsson/foreverbull/strategy/stream"
	"github.com/lhjnilsson/foreverbull/tests/helper"
	mockSupplier "github.com/lhjnilsson/foreverbull/tests/mocks/finance/supplier"
	mockStream "github.com/lhjnilsson/foreverbull/tests/mocks/internal_/stream"
	mockDependency "github.com/lhjnilsson/foreverbull/tests/mocks/strategy/internal_/stream/dependency"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type ExecutionCommandTest struct {
	suite.Suite

	db *pgxpool.Pool
}

func (test *ExecutionCommandTest) SetupSuite() {
	helper.SetupEnvironment(test.T(), &helper.Containers{
		Postgres: true,
	})
}

func (test *ExecutionCommandTest) SetupTest() {
	var err error
	test.db, err = pgxpool.New(context.Background(), environment.GetPostgresURL())
	test.Require().NoError(err)

	err = repository.Recreate(context.TODO(), test.db)
	test.Require().NoError(err)
}

func TestExecutionCommand(t *testing.T) {
	suite.Run(t, new(ExecutionCommandTest))
}

func (test *ExecutionCommandTest) TestRunExecution() {
	m := new(mockStream.Message)
	m.On("MustGet", stream.DBDep).Return(test.db)

	trading := new(mockSupplier.Trading)
	portfolio := finance.Portfolio{
		Cash:           decimal.NewFromFloat(245.5),
		PortfolioValue: decimal.NewFromFloat(123.23),
		Positions: []finance.Position{
			{
				Symbol:    "AAPL",
				CostBasis: decimal.NewFromFloat(10.3),
				Amount:    decimal.NewFromFloat(10.3),
			},
		}}
	trading.On("GetPortfolio").Return(&portfolio, nil)

	m.On("MustGet", dependency.Trading).Return(trading)

	strategies := repository.Strategy{Conn: test.db}
	strategy, err := strategies.Create(context.Background(), "test-strategy", []string{"symbol"}, 0, "worker-service")
	test.NoError(err)

	executions := repository.Execution{Conn: test.db}
	execution, err := executions.Create(context.Background(), strategy.Name, time.Now(), time.Now(), "worker-service")
	test.NoError(err)

	m.On("ParsePayload", mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(0).(*ss.ExecutionRunCommand)
		arg.ExecutionID = execution.ID
	})

	orders := []finance.Order{
		{
			Symbol:         "AAPL",
			Side:           "BUY",
			Filled:         decimal.NewFromFloat(10.3),
			FilledAvgPrice: decimal.NewFromFloat(10.3),
		},
	}

	executionRunner := new(mockDependency.Execution)
	executionRunner.On("Configure", mock.Anything).Return(nil)
	executionRunner.On("Run", mock.Anything, &portfolio).Return(&orders, nil)
	executionRunner.On("Stop", mock.Anything).Return(nil)
	m.On("Call", mock.Anything, dependency.ExecutionRunner).Return(executionRunner, nil)

	err = RunExecution(context.Background(), m)
	test.NoError(err)

	execution, err = executions.Get(context.Background(), execution.ID)
	test.NoError(err)
	test.Equal(portfolio, execution.StartPortfolio)
	test.Equal(orders, execution.PlacedOrders)
}

func (test *ExecutionCommandTest) TestUpdateExecutionStatus() {
	m := new(mockStream.Message)
	m.On("MustGet", stream.DBDep).Return(test.db)

	strategies := repository.Strategy{Conn: test.db}
	strategy, err := strategies.Create(context.Background(), "test-strategy", []string{"symbol"}, 0, "worker-service")
	test.NoError(err)

	executions := repository.Execution{Conn: test.db}
	execution, err := executions.Create(context.Background(), strategy.Name, time.Now(), time.Now(), "worker-service")
	test.NoError(err)

	type TestCase struct {
		Status entity.ExecutionStatusType
		Error  error
	}
	testCases := []TestCase{
		{Status: entity.ExecutionStatusRunning, Error: nil},
		{Status: entity.ExecutionStatusCompleted, Error: nil},
		{Status: entity.ExecutionStatusFailed, Error: errors.New("test error")},
	}
	for _, tc := range testCases {
		m.On("ParsePayload", mock.Anything).Return(nil).Run(func(args mock.Arguments) {
			arg := args.Get(0).(*ss.UpdateExecutionStatusCommand)
			arg.ExecutionID = execution.ID
			arg.Status = tc.Status
			arg.Error = tc.Error
		})

		err := UpdateExecutionStatus(context.Background(), m)
		test.NoError(err)

		execution, err := executions.Get(context.Background(), execution.ID)
		test.NoError(err)
		test.Equal(tc.Status, execution.Statuses[0].Status)
	}
}
