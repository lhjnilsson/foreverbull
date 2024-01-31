package repository

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/strategy/entity"
	"github.com/lhjnilsson/foreverbull/tests/helper"
	"github.com/stretchr/testify/suite"
)

type ExecutionTest struct {
	suite.Suite

	conn *pgxpool.Pool
}

func (suite *ExecutionTest) SetupTest() {
	var err error
	config := helper.TestingConfig(suite.T(), &helper.Containers{
		Postgres: true,
	})
	suite.conn, err = pgxpool.New(context.Background(), config.PostgresURI)
	suite.NoError(err)

	ctx := context.Background()
	err = Recreate(ctx, suite.conn)
	suite.NoError(err)
}

func TestExecution(t *testing.T) {
	suite.Run(t, new(ExecutionTest))
}

func (suite *ExecutionTest) TestCreateExecution() {}

func (suite *ExecutionTest) TestGetByBacktestAndBacktestAtisNull() {
	ctx := context.Background()
	strategies := Strategy{Conn: suite.conn}
	executions := Execution{Conn: suite.conn}

	strategyName := "test"
	backtestName := "backtest"
	strategy := entity.Strategy{
		Name:     &strategyName,
		Backtest: &backtestName,
	}
	err := strategies.Create(ctx, &strategy)
	suite.NoError(err)

	entries, err := executions.GetByBacktestAndBacktestAtisNull(ctx, *strategy.Backtest)
	suite.NoError(err)
	suite.Equal(0, len(*entries))

	_, err = executions.Create(ctx, *strategy.Name)
	suite.NoError(err)

	entries, err = executions.GetByBacktestAndBacktestAtisNull(ctx, *strategy.Backtest)
	suite.NoError(err)
	suite.Equal(1, len(*entries))
}
