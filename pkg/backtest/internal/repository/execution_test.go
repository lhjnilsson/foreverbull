package repository_test

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	common_pb "github.com/lhjnilsson/foreverbull/internal/pb"
	"github.com/lhjnilsson/foreverbull/internal/test_helper"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/repository"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/pb"
	"github.com/stretchr/testify/suite"
)

type ExecutionTest struct {
	suite.Suite

	conn *pgxpool.Pool

	storedBacktest *pb.Backtest
	storedSession  *pb.Session
}

func (test *ExecutionTest) SetupSuite() {
	test_helper.SetupEnvironment(test.T(), &test_helper.Containers{
		Postgres: true,
	})
}

func (test *ExecutionTest) SetupTest() {
	var err error
	test.conn, err = pgxpool.New(context.Background(), environment.GetPostgresURL())
	test.Require().NoError(err)

	err = repository.Recreate(context.Background(), test.conn)
	test.Require().NoError(err)

	ctx := context.Background()
	backtests := &repository.Backtest{Conn: test.conn}
	_, err = backtests.Create(ctx, "backtest", &common_pb.Date{Year: 2024, Month: 0o1, Day: 0o1}, &common_pb.Date{Year: 2024, Month: 0o1, Day: 0o1}, []string{}, nil)
	test.Require().NoError(err)
	test.storedBacktest, err = backtests.Get(ctx, "backtest")
	test.Require().NoError(err)

	sessions := repository.Session{Conn: test.conn}

	test.storedSession, err = sessions.Create(ctx, "backtest")
	test.Require().NoError(err)
}

func (test *ExecutionTest) TearDownTest() {
}

func TestExecutions(t *testing.T) {
	suite.Run(t, new(ExecutionTest))
}

func (test *ExecutionTest) TestCreate() {
	executions := repository.Execution{Conn: test.conn}
	ctx := context.Background()

	for _, end := range []*common_pb.Date{
		{Year: 2024, Month: 0o1, Day: 0o1},
		nil,
	} {
		execution, err := executions.Create(ctx, test.storedSession.Id,
			test.storedBacktest.StartDate, end, test.storedBacktest.Symbols, test.storedBacktest.Benchmark)
		test.Require().NoError(err)
		test.NotNil(execution.Id)
		test.Equal(test.storedSession.Id, execution.Session)
		test.Equal(test.storedBacktest.Symbols, execution.Symbols)
		test.Len(execution.Statuses, 1)
		test.Equal(pb.Execution_Status_CREATED.String(), execution.Statuses[0].Status.String())
		test.Nil(execution.Statuses[0].Error)
		test.NotNil(execution.Statuses[0].OccurredAt)
		test.Equal(end, execution.EndDate)
	}
}

func (test *ExecutionTest) TestGet() {
	executions := repository.Execution{Conn: test.conn}
	ctx := context.Background()
	execution, err := executions.Create(ctx, test.storedSession.Id,
		test.storedBacktest.StartDate, test.storedBacktest.EndDate, test.storedBacktest.Symbols, test.storedBacktest.Benchmark)
	test.Require().NoError(err)
	execution, err = executions.Get(ctx, execution.Id)
	test.Require().NoError(err)
	test.NotNil(execution.Id)
	test.Equal(test.storedSession.Id, execution.Session)
	test.Equal(test.storedBacktest.StartDate, execution.StartDate)
	test.Equal(test.storedBacktest.EndDate, execution.EndDate)
	test.Equal(test.storedBacktest.Symbols, execution.Symbols)
	test.Len(execution.Statuses, 1)
	test.Equal(pb.Execution_Status_CREATED.String(), execution.Statuses[0].Status.String())
	test.Nil(execution.Statuses[0].Error)
	test.NotNil(execution.Statuses[0].OccurredAt)
}

func (test *ExecutionTest) TestGetPeriods() {
	executions := repository.Execution{Conn: test.conn}
	ctx := context.Background()
	execution, err := executions.Create(ctx, test.storedSession.Id,
		test.storedBacktest.StartDate, test.storedBacktest.EndDate, test.storedBacktest.Symbols, test.storedBacktest.Benchmark)
	test.Require().NoError(err)
	periods, err := executions.GetPeriods(ctx, execution.Id)
	test.Require().NoError(err)
	test.NotNil(periods)
}

func (test *ExecutionTest) TestUpdateSimulationDetails() {
	executions := repository.Execution{Conn: test.conn}
	ctx := context.Background()
	execution, err := executions.Create(ctx, test.storedSession.Id,
		test.storedBacktest.StartDate, test.storedBacktest.EndDate, test.storedBacktest.Symbols, test.storedBacktest.Benchmark)
	test.Require().NoError(err)
	// e.StartDate = internal_pb.TimeToProtoTimestamp(&common_pb.Date{Year: 2024, Month: 01, Day: 01})
	// e.EndDate = internal_pb.TimeToProtoTimestamp(&common_pb.Date{Year: 2024, Month: 01, Day: 01})
	execution.Benchmark = func() *string { s := "AAPL"; return &s }()
	execution.Symbols = []string{"AAPL", "MSFT", "TSLA"}
	err = executions.UpdateSimulationDetails(ctx, execution)
	test.Require().NoError(err)
	execution, err = executions.Get(ctx, execution.Id)
	test.Require().NoError(err)
	test.NotNil(execution.Id)
	test.Equal(test.storedSession.Id, execution.Session)
	test.Len(execution.Statuses, 1)
	test.Equal(pb.Execution_Status_CREATED.String(), execution.Statuses[0].Status.String())
	test.Nil(execution.Statuses[0].Error)
	test.NotNil(execution.Statuses[0].OccurredAt)
}

func (test *ExecutionTest) TestUpdateStatus() {
	executions := repository.Execution{Conn: test.conn}
	ctx := context.Background()
	execution, err := executions.Create(ctx, test.storedSession.Id,
		test.storedBacktest.StartDate, test.storedBacktest.EndDate, test.storedBacktest.Symbols, test.storedBacktest.Benchmark)
	test.Require().NoError(err)
	err = executions.UpdateStatus(ctx, execution.Id, pb.Execution_Status_RUNNING, nil)
	test.Require().NoError(err)
	err = executions.UpdateStatus(ctx, execution.Id, pb.Execution_Status_FAILED, errors.New("test"))
	test.Require().NoError(err)

	execution, err = executions.Get(ctx, execution.Id)
	test.Require().NoError(err)
	test.NotNil(execution.Id)
	test.Len(execution.Statuses, 3)
	test.Equal(pb.Execution_Status_FAILED.String(), execution.Statuses[0].Status.String())
	test.Equal("test", *execution.Statuses[0].Error)
	test.NotNil(execution.Statuses[0].OccurredAt)
	test.Equal(pb.Execution_Status_RUNNING.String(), execution.Statuses[1].Status.String())
	test.Nil(execution.Statuses[1].Error)
	test.NotNil(execution.Statuses[1].OccurredAt)
	test.Equal(pb.Execution_Status_CREATED.String(), execution.Statuses[2].Status.String())
	test.Nil(execution.Statuses[2].Error)
	test.NotNil(execution.Statuses[2].OccurredAt)
}

func (test *ExecutionTest) TestList() {
	executions := repository.Execution{Conn: test.conn}
	ctx := context.Background()

	_, err := executions.Create(ctx, test.storedSession.Id,
		test.storedBacktest.StartDate, test.storedBacktest.EndDate, test.storedBacktest.Symbols, test.storedBacktest.Benchmark)
	test.Require().NoError(err)

	_, err = executions.Create(ctx, test.storedSession.Id,
		test.storedBacktest.StartDate, nil, test.storedBacktest.Symbols, test.storedBacktest.Benchmark)
	test.Require().NoError(err)

	storedExecutions, err := executions.List(ctx)
	test.Require().NoError(err)
	test.Len(storedExecutions, 2)
}

func (test *ExecutionTest) TestListBySession() {
	sessions := repository.Session{Conn: test.conn}

	ctx := context.Background()
	session1, err := sessions.Create(ctx, "backtest")
	test.Require().NoError(err)

	session2, err := sessions.Create(ctx, "backtest")
	test.Require().NoError(err)

	executions := repository.Execution{Conn: test.conn}
	_, err = executions.Create(ctx, session1.Id,
		test.storedBacktest.StartDate, test.storedBacktest.EndDate, test.storedBacktest.Symbols, test.storedBacktest.Benchmark)
	test.Require().NoError(err)

	storedExecutions, err := executions.ListBySession(ctx, session1.Id)
	test.Require().NoError(err)
	test.Len(storedExecutions, 1)

	storedExecutions, err = executions.ListBySession(ctx, session2.Id)
	test.Require().NoError(err)
	test.Empty(storedExecutions)
}

func (test *ExecutionTest) TestListByBacktest() {
	sessions := repository.Session{Conn: test.conn}

	ctx := context.Background()
	session, err := sessions.Create(ctx, "backtest")
	test.Require().NoError(err)

	executions := repository.Execution{Conn: test.conn}
	_, err = executions.Create(ctx, session.Id,
		test.storedBacktest.StartDate, test.storedBacktest.EndDate, test.storedBacktest.Symbols, test.storedBacktest.Benchmark)
	test.Require().NoError(err)

	storedExecutions, err := executions.ListByBacktest(ctx, "backtest")
	test.Require().NoError(err)
	test.Len(storedExecutions, 1)
}
