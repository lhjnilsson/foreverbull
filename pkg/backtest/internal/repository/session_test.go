package repository_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/internal/test_helper"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/repository"
	"github.com/lhjnilsson/foreverbull/pkg/pb/backtest"
	"github.com/stretchr/testify/suite"
)

type SessionTest struct {
	suite.Suite

	conn *pgxpool.Pool

	storedBacktest *backtest.Backtest
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
	backtests := &repository.Backtest{Conn: test.conn}
	backtest := backtest.Backtest{
		Name: "backtest",
	}
	test.storedBacktest, err = backtests.Create(ctx, &backtest)
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
	test.Require().NoError(err)
	test.NotNil(session.Id)
	test.Equal("backtest", session.Backtest)
}

func (test *SessionTest) TestGet() {
	sessions := repository.Session{Conn: test.conn}
	ctx := context.Background()
	session, err := sessions.Create(ctx, "backtest")
	test.Require().NoError(err)
	test.NotNil(session.Id)

	session2, err := sessions.Get(ctx, session.Id)
	test.Require().NoError(err)
	test.Equal(session.Id, session2.Id)
	test.Equal(session.Backtest, session2.Backtest)
	test.Equal(session.Port, session2.Port)
}

func (test *SessionTest) TestUpdatePort() {
	sessions := repository.Session{Conn: test.conn}
	ctx := context.Background()
	session, err := sessions.Create(ctx, "backtest")
	test.Require().NoError(err)
	test.NotNil(session.Id)

	err = sessions.UpdatePort(ctx, session.Id, 1337)
	test.Require().NoError(err)

	session2, err := sessions.Get(ctx, session.Id)
	test.Require().NoError(err)
	test.Equal(session.Id, session2.Id)
	test.NotNil(session2.Port)
	test.Equal(int64(1337), *session2.Port)
}

func (test *SessionTest) TestList() {
	sessions := repository.Session{Conn: test.conn}
	ctx := context.Background()
	session1, err := sessions.Create(ctx, "backtest")
	test.Require().NoError(err)
	test.NotNil(session1.Id)

	session2, err := sessions.Create(ctx, "backtest")
	test.Require().NoError(err)
	test.NotNil(session2.Id)

	allSessions, err := sessions.List(ctx)
	test.Require().NoError(err)
	test.Len(allSessions, 2)
	test.Equal(session1.Id, allSessions[0].Id)
	test.Equal(session2.Id, allSessions[1].Id)
}

func (test *SessionTest) TestListByBacktest() {
	sessions := repository.Session{Conn: test.conn}
	ctx := context.Background()
	session1, err := sessions.Create(ctx, "backtest")
	test.Require().NoError(err)
	test.NotNil(session1.Id)

	session2, err := sessions.Create(ctx, "backtest")
	test.Require().NoError(err)
	test.NotNil(session2.Id)

	backtests := &repository.Backtest{Conn: test.conn}
	backtest2 := backtest.Backtest{
		Name: "backtest2",
	}
	test.storedBacktest, err = backtests.Create(ctx, &backtest2)
	test.Require().NoError(err)
	session3, err := sessions.Create(ctx, "backtest2")
	test.Require().NoError(err)
	test.NotNil(session3.Id)

	allSessions, err := sessions.ListByBacktest(ctx, "backtest")
	test.Require().NoError(err)
	test.Len(allSessions, 2)
	test.Equal(session1.Id, allSessions[0].Id)
	test.Equal(session2.Id, allSessions[1].Id)
}

func (test *SessionTest) TestBacktestNotNull() {
	ctx := context.Background()
	test.Require().NoError(repository.MigrateSessionBacktest(ctx, test.conn))
	test.Require().NoError(repository.MigrateSessionBacktest(ctx, test.conn))

	_, err := test.conn.Exec(ctx, `INSERT INTO session (backtest) VALUES (NULL)`)
	test.Require().ErrorContains(err, "session")
}

func (test *SessionTest) TestMigrateSessionBacktestLeavesNull() {
	ctx := context.Background()
	_, err := test.conn.Exec(ctx, `ALTER TABLE session ALTER COLUMN backtest DROP NOT NULL`)
	test.Require().NoError(err)
	var id string
	err = test.conn.QueryRow(ctx, `INSERT INTO session (backtest) VALUES (NULL) RETURNING id`).Scan(&id)
	test.Require().NoError(err)

	err = repository.MigrateSessionBacktest(ctx, test.conn)
	test.Require().ErrorContains(err, "session")

	var backtest *string
	err = test.conn.QueryRow(ctx, `SELECT backtest FROM session WHERE id=$1`, id).Scan(&backtest)
	test.Require().NoError(err)
	test.Nil(backtest)
}
