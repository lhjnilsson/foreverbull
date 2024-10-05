package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	common_pb "github.com/lhjnilsson/foreverbull/internal/pb"
	"github.com/lhjnilsson/foreverbull/internal/test_helper"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/pb"
	"github.com/stretchr/testify/suite"
)

type SessionTest struct {
	suite.Suite

	conn *pgxpool.Pool

	storedBacktest *pb.Backtest
}

func (test *SessionTest) SetupSuite() {
	test_helper.SetupEnvironment(test.T(), &test_helper.Containers{
		Postgres: true,
	})
}

func (test *SessionTest) SetupTest() {
	var err error

	test.conn, err = pgxpool.New(context.Background(), environment.GetPostgresURL())
	test.Require().NoError(err)

	err = Recreate(context.Background(), test.conn)
	test.Require().NoError(err)

	ctx := context.Background()
	b_postgres := &Backtest{Conn: test.conn}
	test.storedBacktest, err = b_postgres.Create(ctx, "backtest",
		&common_pb.Date{Year: 2024, Month: 01, Day: 01},
		&common_pb.Date{Year: 2024, Month: 01, Day: 01}, []string{}, nil)
	test.Require().NoError(err)
}

func (test *SessionTest) TearDownTest() {
}

func TestSessions(t *testing.T) {
	suite.Run(t, new(SessionTest))
}

func (test *SessionTest) TestCreate() {
	db := Session{Conn: test.conn}
	ctx := context.Background()

	s, err := db.Create(ctx, "backtest")
	test.NoError(err)
	test.NotNil(s.Id)
	test.Equal("backtest", s.Backtest)
	test.Len(s.Statuses, 1)
	test.Equal(pb.Session_Status_CREATED.String(), s.Statuses[0].Status.String())
}

func (test *SessionTest) TestGet() {
	db := Session{Conn: test.conn}
	ctx := context.Background()
	s, err := db.Create(ctx, "backtest")
	test.NoError(err)
	test.NotNil(s.Id)

	s2, err := db.Get(ctx, s.Id)
	test.NoError(err)
	test.Equal(s.Id, s2.Id)
	test.Equal(s.Backtest, s2.Backtest)
	test.Equal(s.Port, s2.Port)
	test.Equal(s.Executions, s2.Executions)
	test.Len(s.Statuses, 1)
	test.Equal(pb.Session_Status_CREATED.String(), s.Statuses[0].Status.String())
}

func (test *SessionTest) TestUpdateStatus() {
	db := Session{Conn: test.conn}
	ctx := context.Background()
	s, err := db.Create(ctx, "backtest")
	test.NoError(err)
	test.NotNil(s.Id)

	err = db.UpdateStatus(ctx, s.Id, pb.Session_Status_RUNNING, nil)
	test.NoError(err)

	err = db.UpdateStatus(ctx, s.Id, pb.Session_Status_FAILED, errors.New("test"))
	test.NoError(err)

	s2, err := db.Get(ctx, s.Id)
	test.NoError(err)
	test.Equal(s.Id, s2.Id)
	test.Equal(pb.Session_Status_FAILED.String(), s2.Statuses[0].Status.String())
	test.NotNil(s2.Statuses[0].OccurredAt)
	test.Equal("test", *s2.Statuses[0].Error)
	test.Equal(pb.Session_Status_RUNNING.String(), s2.Statuses[1].Status.String())
	test.NotNil(s2.Statuses[1].OccurredAt)
	test.Equal(pb.Session_Status_CREATED.String(), s2.Statuses[2].Status.String())
	test.NotNil(s2.Statuses[2].OccurredAt)
}

func (test *SessionTest) TestUpdatePort() {
	db := Session{Conn: test.conn}
	ctx := context.Background()
	s, err := db.Create(ctx, "backtest")
	test.NoError(err)
	test.NotNil(s.Id)

	err = db.UpdatePort(ctx, s.Id, 1337)
	test.NoError(err)

	s2, err := db.Get(ctx, s.Id)
	test.NoError(err)
	test.Equal(s.Id, s2.Id)
	test.NotNil(s2.Port)
	test.Equal(int64(1337), *s2.Port)
}

func (test *SessionTest) TestList() {
	db := Session{Conn: test.conn}
	ctx := context.Background()
	s1, err := db.Create(ctx, "backtest")
	test.NoError(err)
	test.NotNil(s1.Id)

	s2, err := db.Create(ctx, "backtest")
	test.NoError(err)
	test.NotNil(s2.Id)

	sessions, err := db.List(ctx)
	test.NoError(err)
	test.Len(sessions, 2)
	test.Equal(s2.Id, sessions[0].Id)
	test.Equal(s1.Id, sessions[1].Id)
}

func (test *SessionTest) TestListByBacktest() {
	db := Session{Conn: test.conn}
	ctx := context.Background()
	s1, err := db.Create(ctx, "backtest")
	test.NoError(err)
	test.NotNil(s1.Id)

	s2, err := db.Create(ctx, "backtest")
	test.NoError(err)
	test.NotNil(s2.Id)

	b_postgres := &Backtest{Conn: test.conn}
	test.storedBacktest, err = b_postgres.Create(ctx, "backtest2",
		&common_pb.Date{Year: 2024, Month: 01, Day: 01},
		&common_pb.Date{Year: 2024, Month: 01, Day: 01}, []string{}, nil)
	test.NoError(err)
	s3, err := db.Create(ctx, "backtest2")
	test.NoError(err)
	test.NotNil(s3.Id)

	sessions, err := db.ListByBacktest(ctx, "backtest")
	test.NoError(err)
	test.Len(sessions, 2)
	test.Equal(s2.Id, sessions[0].Id)
	test.Equal(s1.Id, sessions[1].Id)
}
