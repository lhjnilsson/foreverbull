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
	b_postgres := &repository.Backtest{Conn: test.conn}
	_, err = b_postgres.Create(ctx, "backtest", &common_pb.Date{Year: 2024, Month: 01, Day: 01}, &common_pb.Date{Year: 2024, Month: 01, Day: 01}, []string{}, nil)
	test.Require().NoError(err)
	test.storedBacktest, err = b_postgres.Get(ctx, "backtest")
	test.Require().NoError(err)

	s_postgres := repository.Session{Conn: test.conn}

	test.storedSession, err = s_postgres.Create(ctx, "backtest")
	test.Require().NoError(err)
}

func (test *ExecutionTest) TearDownTest() {
}

func TestExecutions(t *testing.T) {
	suite.Run(t, new(ExecutionTest))
}

func (test *ExecutionTest) TestCreate() {
	db := repository.Execution{Conn: test.conn}
	ctx := context.Background()

	for _, end := range []*common_pb.Date{
		{Year: 2024, Month: 01, Day: 01},
		nil} {
		e, err := db.Create(ctx, test.storedSession.Id,
			test.storedBacktest.StartDate, end, test.storedBacktest.Symbols, test.storedBacktest.Benchmark)
		test.NoError(err)
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
	db := repository.Execution{Conn: test.conn}
	ctx := context.Background()
	e, err := db.Create(ctx, test.storedSession.Id,
		test.storedBacktest.StartDate, test.storedBacktest.EndDate, test.storedBacktest.Symbols, test.storedBacktest.Benchmark)
	test.NoError(err)
	e, err = db.Get(ctx, e.Id)
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
	db := repository.Execution{Conn: test.conn}
	ctx := context.Background()
	e, err := db.Create(ctx, test.storedSession.Id,
		test.storedBacktest.StartDate, test.storedBacktest.EndDate, test.storedBacktest.Symbols, test.storedBacktest.Benchmark)
	test.NoError(err)
	//e.StartDate = internal_pb.TimeToProtoTimestamp(&common_pb.Date{Year: 2024, Month: 01, Day: 01})
	//e.EndDate = internal_pb.TimeToProtoTimestamp(&common_pb.Date{Year: 2024, Month: 01, Day: 01})
	e.Benchmark = func() *string { s := "AAPL"; return &s }()
	e.Symbols = []string{"AAPL", "MSFT", "TSLA"}
	err = db.UpdateSimulationDetails(ctx, e)
	test.NoError(err)
	e, err = db.Get(ctx, e.Id)
	test.NoError(err)
	test.NotNil(e.Id)
	test.Equal(test.storedSession.Id, e.Session)
	test.Len(1, len(e.Statuses))
	test.Equal(pb.Execution_Status_CREATED.String(), e.Statuses[0].Status.String())
	test.Nil(e.Statuses[0].Error)
	test.NotNil(e.Statuses[0].OccurredAt)
}

func (test *ExecutionTest) TestUpdateStatus() {
	db := repository.Execution{Conn: test.conn}
	ctx := context.Background()
	e, err := db.Create(ctx, test.storedSession.Id,
		test.storedBacktest.StartDate, test.storedBacktest.EndDate, test.storedBacktest.Symbols, test.storedBacktest.Benchmark)
	test.NoError(err)
	err = db.UpdateStatus(ctx, e.Id, pb.Execution_Status_RUNNING, nil)
	test.NoError(err)
	err = db.UpdateStatus(ctx, e.Id, pb.Execution_Status_FAILED, errors.New("test"))
	test.NoError(err)

	e, err = db.Get(ctx, e.Id)
	test.NoError(err)
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
	db := repository.Execution{Conn: test.conn}
	ctx := context.Background()

	_, err := db.Create(ctx, test.storedSession.Id,
		test.storedBacktest.StartDate, test.storedBacktest.EndDate, test.storedBacktest.Symbols, test.storedBacktest.Benchmark)
	test.NoError(err)

	_, err = db.Create(ctx, test.storedSession.Id,
		test.storedBacktest.StartDate, nil, test.storedBacktest.Symbols, test.storedBacktest.Benchmark)
	test.NoError(err)

	executions, err := db.List(ctx)
	test.NoError(err)
	test.Len(2, len(executions))
}

func (test *ExecutionTest) TestListBySession() {
	s_postgres := repository.Session{Conn: test.conn}

	ctx := context.Background()
	session1, err := s_postgres.Create(ctx, "backtest")
	test.NoError(err)

	session2, err := s_postgres.Create(ctx, "backtest")
	test.NoError(err)

	db := repository.Execution{Conn: test.conn}
	_, err = db.Create(ctx, session1.Id,
		test.storedBacktest.StartDate, test.storedBacktest.EndDate, test.storedBacktest.Symbols, test.storedBacktest.Benchmark)
	test.NoError(err)

	executions, err := db.ListBySession(ctx, session1.Id)
	test.NoError(err)
	test.Len(1, len(executions))

	executions, err = db.ListBySession(ctx, session2.Id)
	test.NoError(err)
	test.Len(0, len(executions))
}
