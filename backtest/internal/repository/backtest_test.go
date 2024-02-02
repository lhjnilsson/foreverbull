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

type BacktestTest struct {
	suite.Suite

	conn *pgxpool.Pool
}

func (suite *BacktestTest) SetupSuite() {

}

func (suite *BacktestTest) SetupTest() {
	var err error

	helper.SetupEnvironment(suite.T(), &helper.Containers{
		Postgres: true,
	})
	suite.conn, err = pgxpool.New(context.Background(), environment.GetPostgresURL())
	suite.NoError(err)
	err = Recreate(context.Background(), suite.conn)
	suite.NoError(err)
}

func (suite *BacktestTest) TearDownTest() {
}

func TestBacktests(t *testing.T) {
	suite.Run(t, new(BacktestTest))
}

func (suite *BacktestTest) TestCreate() {
	ctx := context.Background()

	db := &Backtest{Conn: suite.conn}
	backtest, err := db.Create(ctx, "backtest", "backtest_service", nil, time.Now(), time.Now(), "XNYS", []string{}, nil)
	suite.NoError(err)
	suite.Equal("backtest", backtest.Name)
	suite.Equal("backtest_service", backtest.BacktestService)
	suite.Len(backtest.Statuses, 1)
	suite.Equal(entity.BacktestStatusCreated, backtest.Statuses[0].Status)
}

func (suite *BacktestTest) TestGet() {
	ctx := context.Background()

	db := &Backtest{Conn: suite.conn}
	_, err := db.Create(ctx, "backtest", "backtest_service", nil, time.Now(), time.Now(), "XNYS", []string{}, nil)
	suite.NoError(err)

	backtest, err := db.Get(ctx, "backtest")
	suite.NoError(err)
	suite.Equal("backtest", backtest.Name)
	suite.Equal("backtest_service", backtest.BacktestService)
	suite.Len(backtest.Statuses, 1)
	suite.Equal(entity.BacktestStatusCreated, backtest.Statuses[0].Status)
}

func (suite *BacktestTest) TestUpdate() {
	ctx := context.Background()

	db := &Backtest{Conn: suite.conn}
	_, err := db.Create(ctx, "backtest", "backtest_service", nil, time.Now(), time.Now(), "XNYS", []string{}, nil)
	suite.NoError(err)

	_, err = db.Update(ctx, "backtest", "backtest_service", nil, time.Now(), time.Now(), "XNYS", []string{"AAPL"}, nil)
	suite.NoError(err)

	backtest, err := db.Get(ctx, "backtest")
	suite.NoError(err)
	suite.Equal("backtest", backtest.Name)
	suite.Equal("backtest_service", backtest.BacktestService)
	// TODO: FIX, Github looses nanoseconds
	// -(time.Time) 2023-10-19 19:53:22.382093481 +0000 UTC
	// +(time.Time) 2023-10-19 19:53:22.382093 +0000 UTC
	//suite.Equal(start, *backtest.Start)
	//suite.Equal(end, *backtest.End)
	suite.Equal([]string{"AAPL"}, backtest.Symbols)
	suite.Nil(backtest.Benchmark)
	suite.Len(backtest.Statuses, 2)
	suite.Equal(entity.BacktestStatusUpdated, backtest.Statuses[0].Status)
}

func (suite *BacktestTest) TestUpdateStatus() {
	ctx := context.Background()

	db := &Backtest{Conn: suite.conn}
	_, err := db.Create(ctx, "backtest", "backtest_service", nil, time.Now(), time.Now(), "XNYS", []string{}, nil)
	suite.NoError(err)

	err = db.UpdateStatus(ctx, "backtest", entity.BacktestStatusIngesting, nil)
	suite.NoError(err)

	err = db.UpdateStatus(ctx, "backtest", entity.BacktestStatusError, errors.New("test"))
	suite.NoError(err)

	backtest, err := db.Get(ctx, "backtest")
	suite.NoError(err)
	suite.Equal("backtest", backtest.Name)
	suite.Equal("backtest_service", backtest.BacktestService)
	suite.Len(backtest.Statuses, 3)
	suite.Equal(entity.BacktestStatusError, backtest.Statuses[0].Status)
	suite.NotNil(backtest.Statuses[0].OccurredAt)
	suite.Equal("test", *backtest.Statuses[0].Error)
	suite.Equal(entity.BacktestStatusIngesting, backtest.Statuses[1].Status)
	suite.NotNil(backtest.Statuses[1].OccurredAt)
	suite.Equal(entity.BacktestStatusCreated, backtest.Statuses[2].Status)
	suite.NotNil(backtest.Statuses[2].OccurredAt)
}

func (suite *BacktestTest) TestList() {
	ctx := context.Background()

	db := &Backtest{Conn: suite.conn}
	_, err := db.Create(ctx, "backtest1", "backtest_service", nil, time.Now(), time.Now(), "XNYS", []string{}, nil)
	suite.NoError(err)
	err = db.UpdateStatus(ctx, "backtest1", entity.BacktestStatusIngesting, nil)
	suite.NoError(err)

	_, err = db.Create(ctx, "backtest2", "backtest_service_2", nil, time.Now(), time.Now(), "XNYS", []string{}, nil)
	suite.NoError(err)
	err = db.UpdateStatus(ctx, "backtest2", entity.BacktestStatusReady, nil)
	suite.NoError(err)

	backtests, err := db.List(ctx)
	suite.NoError(err)
	suite.Len(*backtests, 2)
	suite.Equal("backtest2", (*backtests)[0].Name)
	suite.Equal("backtest_service_2", (*backtests)[0].BacktestService)
	suite.Equal(entity.BacktestStatusReady, (*backtests)[0].Statuses[0].Status)

	suite.Equal("backtest1", (*backtests)[1].Name)
	suite.Equal("backtest_service", (*backtests)[1].BacktestService)
	suite.Equal(entity.BacktestStatusIngesting, (*backtests)[1].Statuses[0].Status)
}

func (suite *BacktestTest) TestDelete() {
	ctx := context.Background()

	db := &Backtest{Conn: suite.conn}
	_, err := db.Create(ctx, "backtest", "backtest_service", nil, time.Now(), time.Now(), "XNYS", []string{}, nil)
	suite.NoError(err)

	err = db.Delete(ctx, "backtest")
	suite.NoError(err)

	backtests, err := db.List(ctx)
	suite.NoError(err)
	suite.Len(*backtests, 0)
}
