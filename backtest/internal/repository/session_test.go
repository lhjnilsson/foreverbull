package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/backtest/entity"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/tests/helper"
	"github.com/stretchr/testify/suite"
)

type SessionTest struct {
	suite.Suite

	conn *pgxpool.Pool

	storedBacktest *entity.Backtest
}

func (suite *SessionTest) SetupTest() {
	var err error

	helper.SetupEnvironment(suite.T(), &helper.Containers{
		Postgres: true,
	})
	suite.conn, err = pgxpool.New(context.Background(), environment.GetPostgresURL())
	suite.NoError(err)

	err = Recreate(context.Background(), suite.conn)
	suite.NoError(err)

	ctx := context.Background()
	b_postgres := &Backtest{Conn: suite.conn}
	suite.storedBacktest, err = b_postgres.Create(ctx, "backtest", "backtest_service", nil, time.Now(), time.Now(), "XNYS", []string{}, nil)
	suite.NoError(err)
}

func (suite *SessionTest) TearDownTest() {
}

func TestSessions(t *testing.T) {
	suite.Run(t, new(SessionTest))
}

func (suite *SessionTest) TestCreate() {
	db := Session{Conn: suite.conn}
	ctx := context.Background()

	manual := []bool{true, false}
	for _, tc := range manual {
		s, err := db.Create(ctx, "backtest", tc)
		suite.NoError(err)
		suite.NotNil(s.ID)
		suite.Equal("backtest", s.Backtest)
		suite.Equal(tc, s.Manual)
		suite.Len(s.Statuses, 1)
		suite.Equal(entity.SessionStatusCreated, s.Statuses[0].Status)
	}
}

func (suite *SessionTest) TestGet() {
	db := Session{Conn: suite.conn}
	ctx := context.Background()
	s, err := db.Create(ctx, "backtest", false)
	suite.NoError(err)
	suite.NotNil(s.ID)

	s2, err := db.Get(ctx, s.ID)
	suite.NoError(err)
	suite.Equal(s.ID, s2.ID)
	suite.Equal(s.Backtest, s2.Backtest)
	suite.Equal(s.Port, s2.Port)
	suite.Equal(s.Executions, s2.Executions)
	suite.Len(s.Statuses, 1)
	suite.Equal(entity.SessionStatusCreated, s.Statuses[0].Status)
}

func (suite *SessionTest) TestUpdateStatus() {
	db := Session{Conn: suite.conn}
	ctx := context.Background()
	s, err := db.Create(ctx, "backtest", false)
	suite.NoError(err)
	suite.NotNil(s.ID)

	err = db.UpdateStatus(ctx, s.ID, entity.SessionStatusRunning, nil)
	suite.NoError(err)

	err = db.UpdateStatus(ctx, s.ID, entity.SessionStatusFailed, errors.New("test"))
	suite.NoError(err)

	s2, err := db.Get(ctx, s.ID)
	suite.NoError(err)
	suite.Equal(s.ID, s2.ID)
	suite.Equal(entity.SessionStatusFailed, s2.Statuses[0].Status)
	suite.NotNil(s2.Statuses[0].OccurredAt)
	suite.Equal("test", *s2.Statuses[0].Error)
	suite.Equal(entity.SessionStatusRunning, s2.Statuses[1].Status)
	suite.NotNil(s2.Statuses[1].OccurredAt)
	suite.Equal(entity.SessionStatusCreated, s2.Statuses[2].Status)
	suite.NotNil(s2.Statuses[2].OccurredAt)
}

func (suite *SessionTest) TestUpdatePort() {
	db := Session{Conn: suite.conn}
	ctx := context.Background()
	s, err := db.Create(ctx, "backtest", false)
	suite.NoError(err)
	suite.NotNil(s.ID)

	err = db.UpdatePort(ctx, s.ID, 1337)
	suite.NoError(err)

	s2, err := db.Get(ctx, s.ID)
	suite.NoError(err)
	suite.Equal(s.ID, s2.ID)
	suite.NotNil(s2.Port)
	suite.Equal(1337, *s2.Port)
}

func (suite *SessionTest) TestList() {
	db := Session{Conn: suite.conn}
	ctx := context.Background()
	s1, err := db.Create(ctx, "backtest", false)
	suite.NoError(err)
	suite.NotNil(s1.ID)
	err = db.UpdateStatus(ctx, s1.ID, entity.SessionStatusRunning, nil)
	suite.NoError(err)

	s2, err := db.Create(ctx, "backtest", false)
	suite.NoError(err)
	suite.NotNil(s2.ID)
	err = db.UpdateStatus(ctx, s2.ID, entity.SessionStatusCompleted, nil)
	suite.NoError(err)

	sessions, err := db.List(ctx)
	suite.NoError(err)
	suite.Len(*sessions, 2)
	suite.Equal(s2.ID, (*sessions)[0].ID)
	suite.Equal(s1.ID, (*sessions)[1].ID)

	suite.Equal(entity.SessionStatusCompleted, (*sessions)[0].Statuses[0].Status)
	suite.Equal(entity.SessionStatusCreated, (*sessions)[0].Statuses[1].Status)
}

func (suite *SessionTest) TestListByBacktest() {
	db := Session{Conn: suite.conn}
	ctx := context.Background()
	s1, err := db.Create(ctx, "backtest", false)
	suite.NoError(err)
	suite.NotNil(s1.ID)
	err = db.UpdateStatus(ctx, s1.ID, entity.SessionStatusRunning, nil)
	suite.NoError(err)

	s2, err := db.Create(ctx, "backtest", false)
	suite.NoError(err)
	suite.NotNil(s2.ID)
	err = db.UpdateStatus(ctx, s2.ID, entity.SessionStatusCompleted, nil)
	suite.NoError(err)

	b_postgres := &Backtest{Conn: suite.conn}
	suite.storedBacktest, err = b_postgres.Create(ctx, "backtest2", "backtest_service", nil, time.Now(), time.Now(), "XNYS", []string{}, nil)
	suite.NoError(err)
	s3, err := db.Create(ctx, "backtest2", false)
	suite.NoError(err)
	suite.NotNil(s3.ID)
	err = db.UpdateStatus(ctx, s3.ID, entity.SessionStatusCompleted, nil)
	suite.NoError(err)

	sessions, err := db.ListByBacktest(ctx, "backtest")
	suite.NoError(err)
	suite.Len(*sessions, 2)
	suite.Equal(s2.ID, (*sessions)[0].ID)
	suite.Equal(s1.ID, (*sessions)[1].ID)

	suite.Equal(entity.SessionStatusCompleted, (*sessions)[0].Statuses[0].Status)
	suite.Equal(entity.SessionStatusCreated, (*sessions)[0].Statuses[1].Status)
}
