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

func (test *BacktestTest) SetupSuite() {

}

func (test *BacktestTest) SetupTest() {
	var err error

	helper.SetupEnvironment(test.T(), &helper.Containers{
		Postgres: true,
	})
	test.conn, err = pgxpool.New(context.Background(), environment.GetPostgresURL())
	test.Require().NoError(err)
	err = Recreate(context.Background(), test.conn)
	test.Require().NoError(err)
}

func (test *BacktestTest) TearDownTest() {
}

func TestBacktests(t *testing.T) {
	suite.Run(t, new(BacktestTest))
}

func (test *BacktestTest) TestCreate() {
	ctx := context.Background()

	db := &Backtest{Conn: test.conn}
	backtest, err := db.Create(ctx, "backtest", "backtest_service", nil, time.Now(), time.Now(), "XNYS", []string{}, nil)
	test.NoError(err)
	test.Equal("backtest", backtest.Name)
	test.Equal("backtest_service", backtest.BacktestService)
	test.Len(backtest.Statuses, 1)
	test.Equal(entity.BacktestStatusCreated, backtest.Statuses[0].Status)
}

func (test *BacktestTest) TestGet() {
	ctx := context.Background()

	db := &Backtest{Conn: test.conn}
	_, err := db.Create(ctx, "backtest", "backtest_service", nil, time.Now(), time.Now(), "XNYS", []string{}, nil)
	test.NoError(err)

	backtest, err := db.Get(ctx, "backtest")
	test.NoError(err)
	test.Equal("backtest", backtest.Name)
	test.Equal("backtest_service", backtest.BacktestService)
	test.Len(backtest.Statuses, 1)
	test.Equal(entity.BacktestStatusCreated, backtest.Statuses[0].Status)
}

func (test *BacktestTest) TestUpdate() {
	ctx := context.Background()

	db := &Backtest{Conn: test.conn}
	_, err := db.Create(ctx, "backtest", "backtest_service", nil, time.Now(), time.Now(), "XNYS", []string{}, nil)
	test.NoError(err)

	_, err = db.Update(ctx, "backtest", "backtest_service", nil, time.Now(), time.Now(), "XNYS", []string{"AAPL"}, nil)
	test.NoError(err)

	backtest, err := db.Get(ctx, "backtest")
	test.NoError(err)
	test.Equal("backtest", backtest.Name)
	test.Equal("backtest_service", backtest.BacktestService)
	// TODO: FIX, Github looses nanoseconds
	// -(time.Time) 2023-10-19 19:53:22.382093481 +0000 UTC
	// +(time.Time) 2023-10-19 19:53:22.382093 +0000 UTC
	//test.Equal(start, *backtest.Start)
	//test.Equal(end, *backtest.End)
	test.Equal([]string{"AAPL"}, backtest.Symbols)
	test.Nil(backtest.Benchmark)
	test.Len(backtest.Statuses, 2)
	test.Equal(entity.BacktestStatusUpdated, backtest.Statuses[0].Status)
}

func (test *BacktestTest) TestUpdateStatus() {
	ctx := context.Background()

	db := &Backtest{Conn: test.conn}
	_, err := db.Create(ctx, "backtest", "backtest_service", nil, time.Now(), time.Now(), "XNYS", []string{}, nil)
	test.NoError(err)

	err = db.UpdateStatus(ctx, "backtest", entity.BacktestStatusIngesting, nil)
	test.NoError(err)

	err = db.UpdateStatus(ctx, "backtest", entity.BacktestStatusError, errors.New("test"))
	test.NoError(err)

	backtest, err := db.Get(ctx, "backtest")
	test.NoError(err)
	test.Equal("backtest", backtest.Name)
	test.Equal("backtest_service", backtest.BacktestService)
	test.Len(backtest.Statuses, 3)
	test.Equal(entity.BacktestStatusError, backtest.Statuses[0].Status)
	test.NotNil(backtest.Statuses[0].OccurredAt)
	test.Equal("test", *backtest.Statuses[0].Error)
	test.Equal(entity.BacktestStatusIngesting, backtest.Statuses[1].Status)
	test.NotNil(backtest.Statuses[1].OccurredAt)
	test.Equal(entity.BacktestStatusCreated, backtest.Statuses[2].Status)
	test.NotNil(backtest.Statuses[2].OccurredAt)
}

func (test *BacktestTest) TestList() {
	ctx := context.Background()

	db := &Backtest{Conn: test.conn}
	_, err := db.Create(ctx, "backtest1", "backtest_service", nil, time.Now(), time.Now(), "XNYS", []string{}, nil)
	test.NoError(err)
	err = db.UpdateStatus(ctx, "backtest1", entity.BacktestStatusIngesting, nil)
	test.NoError(err)

	_, err = db.Create(ctx, "backtest2", "backtest_service_2", nil, time.Now(), time.Now(), "XNYS", []string{}, nil)
	test.NoError(err)
	err = db.UpdateStatus(ctx, "backtest2", entity.BacktestStatusReady, nil)
	test.NoError(err)

	backtests, err := db.List(ctx)
	test.NoError(err)
	test.Len(*backtests, 2)
	test.Equal("backtest2", (*backtests)[0].Name)
	test.Equal("backtest_service_2", (*backtests)[0].BacktestService)
	test.Equal(entity.BacktestStatusReady, (*backtests)[0].Statuses[0].Status)

	test.Equal("backtest1", (*backtests)[1].Name)
	test.Equal("backtest_service", (*backtests)[1].BacktestService)
	test.Equal(entity.BacktestStatusIngesting, (*backtests)[1].Statuses[0].Status)
}

func (test *BacktestTest) TestDelete() {
	ctx := context.Background()

	db := &Backtest{Conn: test.conn}
	_, err := db.Create(ctx, "backtest", "backtest_service", nil, time.Now(), time.Now(), "XNYS", []string{}, nil)
	test.NoError(err)

	err = db.Delete(ctx, "backtest")
	test.NoError(err)

	backtests, err := db.List(ctx)
	test.NoError(err)
	test.Len(*backtests, 0)
}
