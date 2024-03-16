package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/strategy/entity"
	"github.com/lhjnilsson/foreverbull/tests/helper"
	"github.com/stretchr/testify/suite"
)

type ExecutionTest struct {
	suite.Suite

	conn *pgxpool.Pool

	strategy *entity.Strategy
}

func (test *ExecutionTest) SetupSuite() {
	var err error
	helper.SetupEnvironment(test.T(), &helper.Containers{
		Postgres: true,
	})
	test.conn, err = pgxpool.New(context.Background(), environment.GetPostgresURL())
	test.Require().NoError(err)
}

func (test *ExecutionTest) SetupTest() {
	ctx := context.Background()
	err := Recreate(ctx, test.conn)
	test.Require().NoError(err)

	strategies := &Strategy{Conn: test.conn}
	test.strategy, err = strategies.Create(ctx, "test", []string{"AAPL"}, 10, nil)
	test.Require().NoError(err)
}

func TestExecution(t *testing.T) {
	suite.Run(t, new(ExecutionTest))
}

func (test *ExecutionTest) TestCreate() {
	ctx := context.Background()

	db := &Execution{Conn: test.conn}

	execution, err := db.Create(ctx, test.strategy.Name, time.Now(), time.Now(), nil)
	test.NoError(err)
	test.NotNil(execution)
	test.Len(execution.Statuses, 1)
}

func (test *ExecutionTest) TestUpdateStatus() {
	ctx := context.Background()

	db := &Execution{Conn: test.conn}

	execution, err := db.Create(ctx, test.strategy.Name, time.Now(), time.Now(), nil)
	test.NoError(err)

	err = db.UpdateStatus(ctx, execution.ID, entity.ExecutionStatusStarted, nil)
	test.NoError(err)

	execution, err = db.Get(ctx, execution.ID)
	test.NoError(err)
	test.Len(execution.Statuses, 2)
	test.Equal(entity.ExecutionStatusStarted, execution.Statuses[0].Status)

	err = db.UpdateStatus(ctx, execution.ID, entity.ExecutionStatusFailed, errors.New("test"))
	test.NoError(err)

	execution, err = db.Get(ctx, execution.ID)
	test.NoError(err)
	test.Len(execution.Statuses, 3)
	test.Equal(entity.ExecutionStatusFailed, execution.Statuses[0].Status)
	test.Equal("test", *execution.Statuses[0].Error)
}

func (test *ExecutionTest) TestList() {
	ctx := context.Background()

	db := &Execution{Conn: test.conn}

	execution, err := db.Create(ctx, test.strategy.Name, time.Now(), time.Now(), nil)
	test.NoError(err)

	executions, err := db.List(ctx, test.strategy.Name)
	test.NoError(err)
	test.Len(*executions, 1)
	test.Equal(*execution, (*executions)[0])
}
