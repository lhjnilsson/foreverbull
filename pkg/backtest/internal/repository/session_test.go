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

	err = repository.Recreate(context.Background(), test.conn)
	test.Require().NoError(err)

	ctx := context.Background()
	b_postgres := &repository.Backtest{Conn: test.conn}
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
	sessions := repository.Session{Conn: test.conn}
	ctx := context.Background()

	session, err := sessions.Create(ctx, "backtest")
	test.NoError(err)
	test.NotNil(session.Id)
	test.Equal("backtest", session.Backtest)
	test.Len(session.Statuses, 1)
	test.Equal(pb.Session_Status_CREATED.String(), session.Statuses[0].Status.String())
}

func (test *SessionTest) TestGet() {
	sessions := repository.Session{Conn: test.conn}
	ctx := context.Background()
	session, err := sessions.Create(ctx, "backtest")
	test.NoError(err)
	test.NotNil(session.Id)

	session2, err := sessions.Get(ctx, session.Id)
	test.NoError(err)
	test.Equal(session.Id, session2.Id)
	test.Equal(session.Backtest, session2.Backtest)
	test.Equal(session.Port, session2.Port)
	test.Equal(session.Executions, session2.Executions)
	test.Len(session.Statuses, 1)
	test.Equal(pb.Session_Status_CREATED.String(), session.Statuses[0].Status.String())
}

func (test *SessionTest) TestUpdateStatus() {
	sessions := repository.Session{Conn: test.conn}
	ctx := context.Background()
	session1, err := sessions.Create(ctx, "backtest")
	test.NoError(err)
	test.NotNil(session1.Id)

	err = sessions.UpdateStatus(ctx, session1.Id, pb.Session_Status_RUNNING, nil)
	test.NoError(err)

	err = sessions.UpdateStatus(ctx, session1.Id, pb.Session_Status_FAILED, errors.New("test"))
	test.NoError(err)

	session2, err := sessions.Get(ctx, session1.Id)
	test.NoError(err)
	test.Equal(session1.Id, session2.Id)
	test.Equal(pb.Session_Status_FAILED.String(), session2.Statuses[0].Status.String())
	test.NotNil(session2.Statuses[0].OccurredAt)
	test.Equal("test", *session2.Statuses[0].Error)
	test.Equal(pb.Session_Status_RUNNING.String(), session2.Statuses[1].Status.String())
	test.NotNil(session2.Statuses[1].OccurredAt)
	test.Equal(pb.Session_Status_CREATED.String(), session2.Statuses[2].Status.String())
	test.NotNil(session2.Statuses[2].OccurredAt)
}

func (test *SessionTest) TestUpdatePort() {
	sessions := repository.Session{Conn: test.conn}
	ctx := context.Background()
	session, err := sessions.Create(ctx, "backtest")
	test.NoError(err)
	test.NotNil(session.Id)

	err = sessions.UpdatePort(ctx, session.Id, 1337)
	test.NoError(err)

	session2, err := sessions.Get(ctx, session.Id)
	test.NoError(err)
	test.Equal(session.Id, session2.Id)
	test.NotNil(session2.Port)
	test.Equal(int64(1337), *session2.Port)
}

func (test *SessionTest) TestList() {
	sessions := repository.Session{Conn: test.conn}
	ctx := context.Background()
	session1, err := sessions.Create(ctx, "backtest")
	test.NoError(err)
	test.NotNil(session1.Id)

	session2, err := sessions.Create(ctx, "backtest")
	test.NoError(err)
	test.NotNil(session2.Id)

	allSessions, err := sessions.List(ctx)
	test.NoError(err)
	test.Len(allSessions, 2)
	test.Equal(session2.Id, allSessions[0].Id)
	test.Equal(session1.Id, allSessions[1].Id)
}

func (test *SessionTest) TestListByBacktest() {
	sessions := repository.Session{Conn: test.conn}
	ctx := context.Background()
	session1, err := sessions.Create(ctx, "backtest")
	test.NoError(err)
	test.NotNil(session1.Id)

	session2, err := sessions.Create(ctx, "backtest")
	test.NoError(err)
	test.NotNil(session2.Id)

	backtests := &repository.Backtest{Conn: test.conn}
	test.storedBacktest, err = backtests.Create(ctx, "backtest2",
		&common_pb.Date{Year: 2024, Month: 01, Day: 01},
		&common_pb.Date{Year: 2024, Month: 01, Day: 01}, []string{}, nil)
	test.NoError(err)
	session3, err := sessions.Create(ctx, "backtest2")
	test.NoError(err)
	test.NotNil(session3.Id)

	allSessions, err := sessions.ListByBacktest(ctx, "backtest")
	test.NoError(err)
	test.Len(allSessions, 2)
	test.Equal(session2.Id, allSessions[0].Id)
	test.Equal(session1.Id, allSessions[1].Id)
}
