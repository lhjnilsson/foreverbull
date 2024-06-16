package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/internal/test_helper"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/entity"
	"github.com/stretchr/testify/suite"
)

type ExecutionTest struct {
	suite.Suite

	conn *pgxpool.Pool

	storedBacktest *entity.Backtest
	storedSession  *entity.Session
}

func (test *ExecutionTest) SetupSuite() {
}

func (test *ExecutionTest) SetupTest() {
	var err error

	test_helper.SetupEnvironment(test.T(), &test_helper.Containers{
		Postgres: true,
	})
	test.conn, err = pgxpool.New(context.Background(), environment.GetPostgresURL())
	test.Require().NoError(err)

	err = Recreate(context.Background(), test.conn)
	test.Require().NoError(err)

	ctx := context.Background()
	b_postgres := &Backtest{Conn: test.conn}
	_, err = b_postgres.Create(ctx, "backtest", nil, time.Now(), time.Now(), "XNYS", []string{}, nil)
	test.Require().NoError(err)
	test.storedBacktest, err = b_postgres.Get(ctx, "backtest")
	test.Require().NoError(err)

	s_postgres := Session{Conn: test.conn}

	test.storedSession, err = s_postgres.Create(ctx, "backtest", false)
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
	e, err := db.Create(ctx, s.storedSession.ID, s.storedBacktest.Calendar,
		s.storedBacktest.Start, s.storedBacktest.End, s.storedBacktest.Symbols, s.storedBacktest.Benchmark)
	s.NoError(err)
	s.NotNil(e.ID)
	s.Equal(s.storedSession.ID, e.Session)
	s.Equal(s.storedBacktest.Calendar, e.Calendar)
	// TODO: FIX, Github looses nanoseconds
	// -(time.Time) 2023-10-19 19:53:22.382093481 +0000 UTC
	// +(time.Time) 2023-10-19 19:53:22.382093 +0000 UTC
	//test.Equal(start, *backtest.Start)
	//test.Equal(end, *backtest.End)
	//s.Equal(*s.storedBacktest.Start, e.Start)
	//s.Equal(*s.storedBacktest.End, e.End)
	s.Equal(s.storedBacktest.Symbols, e.Symbols)
	s.Equal(1, len(e.Statuses))
	s.Equal(entity.ExecutionStatusCreated, e.Statuses[0].Status)
	s.Nil(e.Statuses[0].Error)
	s.NotNil(e.Statuses[0].OccurredAt)
}

func (s *ExecutionTest) TestGet() {
	db := Execution{Conn: s.conn}
	ctx := context.Background()
	e, err := db.Create(ctx, s.storedSession.ID, s.storedBacktest.Calendar,
		s.storedBacktest.Start, s.storedBacktest.End, s.storedBacktest.Symbols, s.storedBacktest.Benchmark)
	s.NoError(err)
	e, err = db.Get(ctx, e.ID)
	s.NoError(err)
	s.NotNil(e.ID)
	s.Equal(s.storedSession.ID, e.Session)
	s.Equal(s.storedBacktest.Calendar, e.Calendar)
	s.Equal(s.storedBacktest.Start, e.Start)
	s.Equal(s.storedBacktest.End, e.End)
	s.Equal(s.storedBacktest.Symbols, e.Symbols)
	s.Equal(1, len(e.Statuses))
	s.Equal(entity.ExecutionStatusCreated, e.Statuses[0].Status)
	s.Nil(e.Statuses[0].Error)
	s.NotNil(e.Statuses[0].OccurredAt)
}

func (s *ExecutionTest) TestUpdateSimulationDetails() {
	db := Execution{Conn: s.conn}
	ctx := context.Background()
	e, err := db.Create(ctx, s.storedSession.ID, s.storedBacktest.Calendar,
		s.storedBacktest.Start, s.storedBacktest.End, s.storedBacktest.Symbols, s.storedBacktest.Benchmark)
	s.NoError(err)
	e.Calendar = "XNYS"
	e.Start = time.Now().Round(0)
	e.End = time.Now().Round(0)
	e.Benchmark = func() *string { s := "AAPL"; return &s }()
	e.Symbols = []string{"AAPL", "MSFT", "TSLA"}
	err = db.UpdateSimulationDetails(ctx, e)
	s.NoError(err)
	e, err = db.Get(ctx, e.ID)
	s.NoError(err)
	s.NotNil(e.ID)
	s.Equal(s.storedSession.ID, e.Session)
	s.Equal(1, len(e.Statuses))
	s.Equal(entity.ExecutionStatusCreated, e.Statuses[0].Status)
	s.Nil(e.Statuses[0].Error)
	s.NotNil(e.Statuses[0].OccurredAt)
}

func (s *ExecutionTest) TestUpdateStatus() {
	db := Execution{Conn: s.conn}
	ctx := context.Background()
	e, err := db.Create(ctx, s.storedSession.ID, s.storedBacktest.Calendar,
		s.storedBacktest.Start, s.storedBacktest.End, s.storedBacktest.Symbols, s.storedBacktest.Benchmark)
	s.NoError(err)
	err = db.UpdateStatus(ctx, e.ID, entity.ExecutionStatusRunning, nil)
	s.NoError(err)
	err = db.UpdateStatus(ctx, e.ID, entity.ExecutionStatusFailed, errors.New("test"))
	s.NoError(err)

	e, err = db.Get(ctx, e.ID)
	s.NoError(err)
	s.NotNil(e.ID)
	s.Equal(3, len(e.Statuses))
	s.Equal(entity.ExecutionStatusFailed, e.Statuses[0].Status)
	s.Equal("test", *e.Statuses[0].Error)
	s.NotNil(e.Statuses[0].OccurredAt)
	s.Equal(entity.ExecutionStatusRunning, e.Statuses[1].Status)
	s.Nil(e.Statuses[1].Error)
	s.NotNil(e.Statuses[1].OccurredAt)
	s.Equal(entity.ExecutionStatusCreated, e.Statuses[2].Status)
	s.Nil(e.Statuses[2].Error)
	s.NotNil(e.Statuses[2].OccurredAt)
}

func (s *ExecutionTest) TestList() {
	db := Execution{Conn: s.conn}
	ctx := context.Background()

	e, err := db.Create(ctx, s.storedSession.ID, s.storedBacktest.Calendar,
		s.storedBacktest.Start, s.storedBacktest.End, s.storedBacktest.Symbols, s.storedBacktest.Benchmark)
	s.NoError(err)
	err = db.UpdateStatus(ctx, e.ID, entity.ExecutionStatusRunning, nil)
	s.NoError(err)
	err = db.UpdateStatus(ctx, e.ID, entity.ExecutionStatusCompleted, nil)
	s.NoError(err)

	e, err = db.Create(ctx, s.storedSession.ID, s.storedBacktest.Calendar,
		s.storedBacktest.Start, s.storedBacktest.End, s.storedBacktest.Symbols, s.storedBacktest.Benchmark)
	s.NoError(err)
	err = db.UpdateStatus(ctx, e.ID, entity.ExecutionStatusRunning, nil)
	s.NoError(err)
	err = db.UpdateStatus(ctx, e.ID, entity.ExecutionStatusFailed, errors.New("test"))
	s.NoError(err)

	executions, err := db.List(ctx)
	s.NoError(err)
	s.Equal(2, len(*executions))

	s.Equal(3, len((*executions)[0].Statuses))
	s.Equal(entity.ExecutionStatusFailed, (*executions)[0].Statuses[0].Status)
	s.Equal(entity.ExecutionStatusRunning, (*executions)[0].Statuses[1].Status)
	s.Equal(entity.ExecutionStatusCreated, (*executions)[0].Statuses[2].Status)

	s.Equal(3, len((*executions)[1].Statuses))
	s.Equal(entity.ExecutionStatusCompleted, (*executions)[1].Statuses[0].Status)
	s.Equal(entity.ExecutionStatusRunning, (*executions)[1].Statuses[1].Status)
	s.Equal(entity.ExecutionStatusCreated, (*executions)[1].Statuses[2].Status)
}

func (s *ExecutionTest) TestListBySession() {
	s_postgres := Session{Conn: s.conn}

	ctx := context.Background()
	session1, err := s_postgres.Create(ctx, "backtest", false)
	s.NoError(err)

	session2, err := s_postgres.Create(ctx, "backtest", false)
	s.NoError(err)

	db := Execution{Conn: s.conn}
	_, err = db.Create(ctx, session1.ID, s.storedBacktest.Calendar,
		s.storedBacktest.Start, s.storedBacktest.End, s.storedBacktest.Symbols, s.storedBacktest.Benchmark)
	s.NoError(err)

	executions, err := db.ListBySession(ctx, session1.ID)
	s.NoError(err)
	s.Equal(1, len(*executions))

	executions, err = db.ListBySession(ctx, session2.ID)
	s.NoError(err)
	s.Equal(0, len(*executions))
}
