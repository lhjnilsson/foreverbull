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

type SessionTest struct {
	suite.Suite

	conn *pgxpool.Pool

	storedBacktest *entity.Backtest
}

func (test *SessionTest) SetupTest() {
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
	test.storedBacktest, err = b_postgres.Create(ctx, "backtest", nil, time.Now(), time.Now(), "XNYS", []string{}, nil)
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

	manual := []bool{true, false}
	for _, tc := range manual {
		s, err := db.Create(ctx, "backtest", tc)
		test.NoError(err)
		test.NotNil(s.ID)
		test.Equal("backtest", s.Backtest)
		test.Equal(tc, s.Manual)
		test.Len(s.Statuses, 1)
		test.Equal(entity.SessionStatusCreated, s.Statuses[0].Status)
	}
}

func (test *SessionTest) TestGet() {
	db := Session{Conn: test.conn}
	ctx := context.Background()
	s, err := db.Create(ctx, "backtest", false)
	test.NoError(err)
	test.NotNil(s.ID)

	s2, err := db.Get(ctx, s.ID)
	test.NoError(err)
	test.Equal(s.ID, s2.ID)
	test.Equal(s.Backtest, s2.Backtest)
	test.Equal(s.Port, s2.Port)
	test.Equal(s.Executions, s2.Executions)
	test.Len(s.Statuses, 1)
	test.Equal(entity.SessionStatusCreated, s.Statuses[0].Status)
}

func (test *SessionTest) TestUpdateStatus() {
	db := Session{Conn: test.conn}
	ctx := context.Background()
	s, err := db.Create(ctx, "backtest", false)
	test.NoError(err)
	test.NotNil(s.ID)

	err = db.UpdateStatus(ctx, s.ID, entity.SessionStatusRunning, nil)
	test.NoError(err)

	err = db.UpdateStatus(ctx, s.ID, entity.SessionStatusFailed, errors.New("test"))
	test.NoError(err)

	s2, err := db.Get(ctx, s.ID)
	test.NoError(err)
	test.Equal(s.ID, s2.ID)
	test.Equal(entity.SessionStatusFailed, s2.Statuses[0].Status)
	test.NotNil(s2.Statuses[0].OccurredAt)
	test.Equal("test", *s2.Statuses[0].Error)
	test.Equal(entity.SessionStatusRunning, s2.Statuses[1].Status)
	test.NotNil(s2.Statuses[1].OccurredAt)
	test.Equal(entity.SessionStatusCreated, s2.Statuses[2].Status)
	test.NotNil(s2.Statuses[2].OccurredAt)
}

func (test *SessionTest) TestUpdatePort() {
	db := Session{Conn: test.conn}
	ctx := context.Background()
	s, err := db.Create(ctx, "backtest", false)
	test.NoError(err)
	test.NotNil(s.ID)

	err = db.UpdatePort(ctx, s.ID, 1337)
	test.NoError(err)

	s2, err := db.Get(ctx, s.ID)
	test.NoError(err)
	test.Equal(s.ID, s2.ID)
	test.NotNil(s2.Port)
	test.Equal(1337, *s2.Port)
}

func (test *SessionTest) TestList() {
	db := Session{Conn: test.conn}
	ctx := context.Background()
	s1, err := db.Create(ctx, "backtest", false)
	test.NoError(err)
	test.NotNil(s1.ID)
	err = db.UpdateStatus(ctx, s1.ID, entity.SessionStatusRunning, nil)
	test.NoError(err)

	s2, err := db.Create(ctx, "backtest", false)
	test.NoError(err)
	test.NotNil(s2.ID)
	err = db.UpdateStatus(ctx, s2.ID, entity.SessionStatusCompleted, nil)
	test.NoError(err)

	sessions, err := db.List(ctx)
	test.NoError(err)
	test.Len(*sessions, 2)
	test.Equal(s2.ID, (*sessions)[0].ID)
	test.Equal(s1.ID, (*sessions)[1].ID)

	test.Equal(entity.SessionStatusCompleted, (*sessions)[0].Statuses[0].Status)
	test.Equal(entity.SessionStatusCreated, (*sessions)[0].Statuses[1].Status)
}

func (test *SessionTest) TestListByBacktest() {
	db := Session{Conn: test.conn}
	ctx := context.Background()
	s1, err := db.Create(ctx, "backtest", false)
	test.NoError(err)
	test.NotNil(s1.ID)
	err = db.UpdateStatus(ctx, s1.ID, entity.SessionStatusRunning, nil)
	test.NoError(err)

	s2, err := db.Create(ctx, "backtest", false)
	test.NoError(err)
	test.NotNil(s2.ID)
	err = db.UpdateStatus(ctx, s2.ID, entity.SessionStatusCompleted, nil)
	test.NoError(err)

	b_postgres := &Backtest{Conn: test.conn}
	test.storedBacktest, err = b_postgres.Create(ctx, "backtest2", nil, time.Now(), time.Now(), "XNYS", []string{}, nil)
	test.NoError(err)
	s3, err := db.Create(ctx, "backtest2", false)
	test.NoError(err)
	test.NotNil(s3.ID)
	err = db.UpdateStatus(ctx, s3.ID, entity.SessionStatusCompleted, nil)
	test.NoError(err)

	sessions, err := db.ListByBacktest(ctx, "backtest")
	test.NoError(err)
	test.Len(*sessions, 2)
	test.Equal(s2.ID, (*sessions)[0].ID)
	test.Equal(s1.ID, (*sessions)[1].ID)

	test.Equal(entity.SessionStatusCompleted, (*sessions)[0].Statuses[0].Status)
	test.Equal(entity.SessionStatusCreated, (*sessions)[0].Statuses[1].Status)
}
