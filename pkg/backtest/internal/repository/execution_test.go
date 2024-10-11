package repository

import (
	"context"
	"errors"
	"testing"

	common_pb "github.com/lhjnilsson/foreverbull/internal/pb"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/internal/test_helper"
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

	err = Recreate(context.Background(), test.conn)
	test.Require().NoError(err)

	ctx := context.Background()
	b_postgres := &Backtest{Conn: test.conn}
	_, err = b_postgres.Create(ctx, "backtest", &common_pb.Date{Year: 2024, Month: 01, Day: 01}, &common_pb.Date{Year: 2024, Month: 01, Day: 01}, []string{}, nil)
	test.Require().NoError(err)
	test.storedBacktest, err = b_postgres.Get(ctx, "backtest")
	test.Require().NoError(err)

	s_postgres := Session{Conn: test.conn}

	test.storedSession, err = s_postgres.Create(ctx, "backtest")
	test.Require().NoError(err)
}

func (test *ExecutionTest) TearDownTest() {
}

func TestExecutions(t *testing.T) {
	suite.Run(t, new(ExecutionTest))
}

func (s *ExecutionTest) TestCreate() {
	db := Execution{Conn: s.conn}
	ctx := context.Background()
	for _, end := range []*common_pb.Date{
		{Year: 2024, Month: 01, Day: 01},
		nil} {
		e, err := db.Create(ctx, s.storedSession.Id,
			s.storedBacktest.StartDate, end, s.storedBacktest.Symbols, s.storedBacktest.Benchmark)
		s.NoError(err)
		s.NotNil(e.Id)
		s.Equal(s.storedSession.Id, e.Session)
		s.Equal(s.storedBacktest.Symbols, e.Symbols)
		s.Equal(1, len(e.Statuses))
		s.Equal(pb.Execution_Status_CREATED.String(), e.Statuses[0].Status.String())
		s.Nil(e.Statuses[0].Error)
		s.NotNil(e.Statuses[0].OccurredAt)
		s.Equal(end, e.EndDate)
	}
}

func (s *ExecutionTest) TestGet() {
	db := Execution{Conn: s.conn}
	ctx := context.Background()
	e, err := db.Create(ctx, s.storedSession.Id,
		s.storedBacktest.StartDate, s.storedBacktest.EndDate, s.storedBacktest.Symbols, s.storedBacktest.Benchmark)
	s.NoError(err)
	e, err = db.Get(ctx, e.Id)
	s.NoError(err)
	s.NotNil(e.Id)
	s.Equal(s.storedSession.Id, e.Session)
	s.Equal(s.storedBacktest.StartDate, e.StartDate)
	s.Equal(s.storedBacktest.EndDate, e.EndDate)
	s.Equal(s.storedBacktest.Symbols, e.Symbols)
	s.Equal(1, len(e.Statuses))
	s.Equal(pb.Execution_Status_CREATED.String(), e.Statuses[0].Status.String())
	s.Nil(e.Statuses[0].Error)
	s.NotNil(e.Statuses[0].OccurredAt)
}

func (s *ExecutionTest) TestUpdateSimulationDetails() {
	db := Execution{Conn: s.conn}
	ctx := context.Background()
	e, err := db.Create(ctx, s.storedSession.Id,
		s.storedBacktest.StartDate, s.storedBacktest.EndDate, s.storedBacktest.Symbols, s.storedBacktest.Benchmark)
	s.NoError(err)
	//e.StartDate = internal_pb.TimeToProtoTimestamp(&common_pb.Date{Year: 2024, Month: 01, Day: 01})
	//e.EndDate = internal_pb.TimeToProtoTimestamp(&common_pb.Date{Year: 2024, Month: 01, Day: 01})
	e.Benchmark = func() *string { s := "AAPL"; return &s }()
	e.Symbols = []string{"AAPL", "MSFT", "TSLA"}
	err = db.UpdateSimulationDetails(ctx, e)
	s.NoError(err)
	e, err = db.Get(ctx, e.Id)
	s.NoError(err)
	s.NotNil(e.Id)
	s.Equal(s.storedSession.Id, e.Session)
	s.Equal(1, len(e.Statuses))
	s.Equal(pb.Execution_Status_CREATED.String(), e.Statuses[0].Status.String())
	s.Nil(e.Statuses[0].Error)
	s.NotNil(e.Statuses[0].OccurredAt)
}

func (s *ExecutionTest) TestUpdateStatus() {
	db := Execution{Conn: s.conn}
	ctx := context.Background()
	e, err := db.Create(ctx, s.storedSession.Id,
		s.storedBacktest.StartDate, s.storedBacktest.EndDate, s.storedBacktest.Symbols, s.storedBacktest.Benchmark)
	s.NoError(err)
	err = db.UpdateStatus(ctx, e.Id, pb.Execution_Status_RUNNING, nil)
	s.NoError(err)
	err = db.UpdateStatus(ctx, e.Id, pb.Execution_Status_FAILED, errors.New("test"))
	s.NoError(err)

	e, err = db.Get(ctx, e.Id)
	s.NoError(err)
	s.NotNil(e.Id)
	s.Equal(3, len(e.Statuses))
	s.Equal(pb.Execution_Status_FAILED.String(), e.Statuses[0].Status.String())
	s.Equal("test", *e.Statuses[0].Error)
	s.NotNil(e.Statuses[0].OccurredAt)
	s.Equal(pb.Execution_Status_RUNNING.String(), e.Statuses[1].Status.String())
	s.Nil(e.Statuses[1].Error)
	s.NotNil(e.Statuses[1].OccurredAt)
	s.Equal(pb.Execution_Status_CREATED.String(), e.Statuses[2].Status.String())
	s.Nil(e.Statuses[2].Error)
	s.NotNil(e.Statuses[2].OccurredAt)
}

func (s *ExecutionTest) TestList() {
	db := Execution{Conn: s.conn}
	ctx := context.Background()

	_, err := db.Create(ctx, s.storedSession.Id,
		s.storedBacktest.StartDate, s.storedBacktest.EndDate, s.storedBacktest.Symbols, s.storedBacktest.Benchmark)
	s.NoError(err)

	_, err = db.Create(ctx, s.storedSession.Id,
		s.storedBacktest.StartDate, nil, s.storedBacktest.Symbols, s.storedBacktest.Benchmark)
	s.NoError(err)

	executions, err := db.List(ctx)
	s.NoError(err)
	s.Equal(2, len(executions))
}

func (s *ExecutionTest) TestListBySession() {
	s_postgres := Session{Conn: s.conn}

	ctx := context.Background()
	session1, err := s_postgres.Create(ctx, "backtest")
	s.NoError(err)

	session2, err := s_postgres.Create(ctx, "backtest")
	s.NoError(err)

	db := Execution{Conn: s.conn}
	_, err = db.Create(ctx, session1.Id,
		s.storedBacktest.StartDate, s.storedBacktest.EndDate, s.storedBacktest.Symbols, s.storedBacktest.Benchmark)
	s.NoError(err)

	executions, err := db.ListBySession(ctx, session1.Id)
	s.NoError(err)
	s.Equal(1, len(executions))

	executions, err = db.ListBySession(ctx, session2.Id)
	s.NoError(err)
	s.Equal(0, len(executions))
}
