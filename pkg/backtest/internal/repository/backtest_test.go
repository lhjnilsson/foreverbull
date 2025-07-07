package repository_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/internal/test_helper"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/repository"
	"github.com/stretchr/testify/suite"
)

type BacktestTest struct {
	suite.Suite

	conn *pgxpool.Pool
}

func (test *BacktestTest) SetupSuite() {
	test_helper.SetupEnvironment(test.T(), &test_helper.Containers{
		Postgres: true,
	})
}

func (test *BacktestTest) SetupTest() {
	var err error

	test.conn, err = pgxpool.New(context.Background(), environment.GetPostgresURL())
	test.Require().NoError(err)
	err = repository.Recreate(context.Background(), test.conn)
	test.Require().NoError(err)
}

func (test *BacktestTest) TearDownTest() {
}

func TestBacktests(t *testing.T) {
	suite.Run(t, new(BacktestTest))
}

func (test *BacktestTest) TestCreate() {
	ctx := context.Background()

	for index, benchmark := range []*string{nil, func() *string { s := "benchmark"; return &s }()} {
		test.Run(fmt.Sprintf("create backtest %d with benchmark %v", index, benchmark), func() {
			backtests := &repository.Backtest{Conn: test.conn}
			backtest, err := backtests.Create(ctx,
				fmt.Sprintf("backtest_bench_%d", index),
				benchmark,
			)
			test.Require().NoError(err)
			test.Equal(fmt.Sprintf("backtest_bench_%d", index), backtest.Name)
			if benchmark == nil {
				test.Nil(backtest.Benchmark)
			} else {
				test.Equal(*benchmark, *backtest.Benchmark)
			}
		})
	}
}

func (test *BacktestTest) TestGet() {
	ctx := context.Background()

	test.Run("not found", func() {
		backtests := &repository.Backtest{Conn: test.conn}
		backtest, err := backtests.Get(ctx, "backtest")
		test.Require().Error(err)
		test.Nil(backtest)
		test.ErrorContains(err, pgx.ErrNoRows.Error())
	})
	test.Run("found", func() {
		backtests := &repository.Backtest{Conn: test.conn}
		_, err := backtests.Create(ctx, "backtest", nil)
		test.Require().NoError(err)

		backtest, err := backtests.Get(ctx, "backtest")
		test.Require().NoError(err)
		test.Equal("backtest", backtest.Name)
		test.Nil(backtest.Benchmark)
	})
}

func (test *BacktestTest) TestUpdate() {
	ctx := context.Background()

	backtests := &repository.Backtest{Conn: test.conn}
	_, err := backtests.Create(ctx, "backtest", nil)
	test.Require().NoError(err)

	benchmark := "benchmark"
	_, err = backtests.Update(ctx, "backtest", &benchmark)
	test.Require().NoError(err)

	backtest, err := backtests.Get(ctx, "backtest")
	test.Require().NoError(err)
	test.Equal("backtest", backtest.Name)
	test.Equal(benchmark, *backtest.Benchmark)
}

func (test *BacktestTest) TestList() {
	ctx := context.Background()

	backtests := &repository.Backtest{Conn: test.conn}
	_, err := backtests.Create(ctx, "backtest1", nil)
	test.Require().NoError(err)

	_, err = backtests.Create(ctx, "backtest2", nil)
	test.Require().NoError(err)

	storedBacktests, err := backtests.List(ctx)
	test.Require().NoError(err)
	test.Len(storedBacktests, 2)
}

func (test *BacktestTest) TestDelete() {
	ctx := context.Background()

	backtests := &repository.Backtest{Conn: test.conn}
	_, err := backtests.Create(ctx, "backtest", nil)
	test.Require().NoError(err)

	err = backtests.Delete(ctx, "backtest")
	test.Require().NoError(err)

	storedBacktests, err := backtests.List(ctx)
	test.Require().NoError(err)
	test.Empty(storedBacktests)
}
