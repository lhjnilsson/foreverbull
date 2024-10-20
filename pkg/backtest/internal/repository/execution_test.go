package repository_test

import (
	"context"
	"errors"
	"testing"

	common_pb "github.com/lhjnilsson/foreverbull/internal/pb"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/environment"
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
		e, err := executions.Create(ctx, test.storedSession.Id,
			test.storedBacktest.StartDate, end, test.storedBacktest.Symbols, test.storedBacktest.Benchmark)
		test.Require().NoError(err)
		test.NotNil(e.Id)
		test.Equal(test.storedSession.Id, e.Session)
		test.Equal(test.storedBacktest.Symbols, e.Symbols)
		test.Len(1, len(e.Statuses))
		test.Equal(pb.Execution_Status_CREATED.String(), e.Statuses[0].Status.String())
		test.Nil(e.Statuses[0].Error)
		test.NotNil(e.Statuses[0].OccurredAt)
		test.Equal(end, e.EndDate)
	}
}

func (test *ExecutionTest) TestGet() {
	executions := repository.Execution{Conn: test.conn}
	ctx := context.Background()
	e, err := executions.Create(ctx, test.storedSession.Id,
		test.storedBacktest.StartDate, test.storedBacktest.EndDate, test.storedBacktest.Symbols, test.storedBacktest.Benchmark)
	test.Require().NoError(err)
	e, err = executions.Get(ctx, e.Id)
	test.NoError(err)
	test.NotNil(e.Id)
	test.Equal(test.storedSession.Id, e.Session)
	test.Equal(test.storedBacktest.StartDate, e.StartDate)
	test.Equal(test.storedBacktest.EndDate, e.EndDate)
	test.Equal(test.storedBacktest.Symbols, e.Symbols)
	test.Len(1, len(e.Statuses))
	test.Equal(pb.Execution_Status_CREATED.String(), e.Statuses[0].Status.String())
	test.Nil(e.Statuses[0].Error)
	test.NotNil(e.Statuses[0].OccurredAt)
}

func (test *ExecutionTest) TestUpdateSimulationDetails() {
	executions := repository.Execution{Conn: test.conn}
	ctx := context.Background()
	e, err := executions.Create(ctx, test.storedSession.Id,
		test.storedBacktest.StartDate, test.storedBacktest.EndDate, test.storedBacktest.Symbols, test.storedBacktest.Benchmark)
	test.Require().NoError(err)
	// e.StartDate = internal_pb.TimeToProtoTimestamp(&common_pb.Date{Year: 2024, Month: 01, Day: 01})
	// e.EndDate = internal_pb.TimeToProtoTimestamp(&common_pb.Date{Year: 2024, Month: 01, Day: 01})
	e.Benchmark = func() *string { s := "AAPL"; return &s }()
	e.Symbols = []string{"AAPL", "MSFT", "TSLA"}
	err = executions.UpdateSimulationDetails(ctx, e)
	test.Require().NoError(err)
	e, err = executions.Get(ctx, e.Id)
	test.Require().NoError(err)
	test.NotNil(e.Id)
	test.Equal(test.storedSession.Id, e.Session)
	test.Len(1, len(e.Statuses))
	test.Equal(pb.Execution_Status_CREATED.String(), e.Statuses[0].Status.String())
	test.Nil(e.Statuses[0].Error)
	test.NotNil(e.Statuses[0].OccurredAt)
}

func (test *ExecutionTest) TestUpdateStatus() {
	executions := repository.Execution{Conn: test.conn}
	ctx := context.Background()
	e, err := executions.Create(ctx, test.storedSession.Id,
		test.storedBacktest.StartDate, test.storedBacktest.EndDate, test.storedBacktest.Symbols, test.storedBacktest.Benchmark)
	test.Require().NoError(err)
	err = executions.UpdateStatus(ctx, e.Id, pb.Execution_Status_RUNNING, nil)
	test.Require().NoError(err)
	err = executions.UpdateStatus(ctx, e.Id, pb.Execution_Status_FAILED, errors.New("test"))
	test.Require().NoError(err)

	e, err = executions.Get(ctx, e.Id)
	test.Require().NoError(err)
	test.NotNil(e.Id)
	test.Len(3, len(e.Statuses))
	test.Equal(pb.Execution_Status_FAILED.String(), e.Statuses[0].Status.String())
	test.Equal("test", *e.Statuses[0].Error)
	test.NotNil(e.Statuses[0].OccurredAt)
	test.Equal(pb.Execution_Status_RUNNING.String(), e.Statuses[1].Status.String())
	test.Nil(e.Statuses[1].Error)
	test.NotNil(e.Statuses[1].OccurredAt)
	test.Equal(pb.Execution_Status_CREATED.String(), e.Statuses[2].Status.String())
	test.Nil(e.Statuses[2].Error)
	test.NotNil(e.Statuses[2].OccurredAt)
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
	test.Len(2, len(storedExecutions))
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
	test.Len(1, len(storedExecutions))

	storedExecutions, err = executions.ListBySession(ctx, session2.Id)
	test.Require().NoError(err)
	test.Len(0, len(storedExecutions))
}
