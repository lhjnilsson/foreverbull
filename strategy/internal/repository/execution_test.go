package repository

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/strategy/entity"
	"github.com/lhjnilsson/foreverbull/tests/helper"
	"github.com/stretchr/testify/suite"
)

type ExecutionTest struct {
	suite.Suite

	conn *pgxpool.Pool
}

func (test *ExecutionTest) SetupTest() {
	var err error
	helper.SetupEnvironment(test.T(), &helper.Containers{
		Postgres: true,
	})
	test.conn, err = pgxpool.New(context.Background(), environment.GetPostgresURL())
	test.Require().NoError(err)

	ctx := context.Background()
	err = Recreate(ctx, test.conn)
	test.Require().NoError(err)
}

func TestExecution(t *testing.T) {
	suite.Run(t, new(ExecutionTest))
}

func (test *ExecutionTest) TestCreateExecution() {}

func (test *ExecutionTest) TestGetByBacktestAndBacktestAtisNull() {
	ctx := context.Background()
	strategies := Strategy{Conn: test.conn}
	executions := Execution{Conn: test.conn}

	strategyName := "test"
	backtestName := "backtest"
	strategy := entity.Strategy{
		Name:     &strategyName,
		Backtest: &backtestName,
	}
	err := strategies.Create(ctx, &strategy)
	test.NoError(err)

	entries, err := executions.GetByBacktestAndBacktestAtisNull(ctx, *strategy.Backtest)
	test.NoError(err)
	test.Equal(0, len(*entries))

	_, err = executions.Create(ctx, *strategy.Name)
	test.NoError(err)

	entries, err = executions.GetByBacktestAndBacktestAtisNull(ctx, *strategy.Backtest)
	test.NoError(err)
	test.Equal(1, len(*entries))
}
