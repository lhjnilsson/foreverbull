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
	"github.com/lhjnilsson/foreverbull/pkg/pb"
	"github.com/lhjnilsson/foreverbull/pkg/pb/backtest"
	"github.com/stretchr/testify/suite"
)

type BacktestTest struct {
	suite.Suite

	conn     *pgxpool.Pool
	uut      repository.Backtest
	backtest *backtest.Backtest
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
	test.uut = repository.Backtest{Conn: test.conn}
	test.backtest = &backtest.Backtest{
		Name:      "test_backtest",
		Benchmark: func() *string { s := "AAPL"; return &s }(),
	}
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
			test.backtest.Benchmark = benchmark
			test.backtest.Name = fmt.Sprintf("backtest_bench_%d", index)
			backtest, err := test.uut.Create(ctx, test.backtest)
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
		_, err := backtests.Create(ctx, test.backtest)
		test.Require().NoError(err)

		backtest, err := backtests.Get(ctx, test.backtest.Name)
		test.Require().NoError(err)
		test.Equal(test.backtest.Name, backtest.Name)
	})
}

func (test *BacktestTest) TestUpdateBenchmark() {
	ctx := context.Background()

	backtests := &repository.Backtest{Conn: test.conn}
	_, err := backtests.Create(ctx, test.backtest)
	test.Require().NoError(err)

	benchmark := "benchmark"
	_, err = backtests.UpdateBenchmark(ctx, test.backtest.Name, &benchmark)
	test.Require().NoError(err)

	backtest, err := backtests.Get(ctx, test.backtest.Name)
	test.Require().NoError(err)
	test.Equal(test.backtest.Name, backtest.Name)
	test.Equal(benchmark, *backtest.Benchmark)
}

func (test *BacktestTest) TestUpdatePeriod() {
	ctx := context.Background()

	backtests := &repository.Backtest{Conn: test.conn}
	test.backtest.Periods = []*backtest.Backtest_Period{
		{
			StartDate: &pb.Date{Year: 2024, Month: 1, Day: 1},
			EndDate:   &pb.Date{Year: 2024, Month: 12, Day: 31},
		},
	}

	_, err := backtests.Create(ctx, test.backtest)
	test.Require().NoError(err)
	period := &backtest.Backtest_Period{
		StartDate: &pb.Date{Year: 2024, Month: 1, Day: 1},
		EndDate:   &pb.Date{Year: 2024, Month: 12, Day: 31},
		Assets: []*backtest.Backtest_Period_Asset{
			{
				Symbol:  "AAPL",
				Enabled: true,
			},
		},
	}
	err = backtests.UpdatePeriod(ctx, test.backtest.Name, period)
	test.Require().NoError(err)

	backtest, err := backtests.Get(ctx, test.backtest.Name)
	test.Require().NoError(err)
	test.Equal(test.backtest.Name, backtest.Name)
	test.Equal(period.StartDate, backtest.Periods[0].StartDate)
	test.Equal(period.EndDate, backtest.Periods[0].EndDate)
	test.Equal(period.Assets[0].Symbol, backtest.Periods[0].Assets[0].Symbol)
}

func (test *BacktestTest) TestUpdatePeriodAsset() {
	ctx := context.Background()

	backtests := &repository.Backtest{Conn: test.conn}
	test.backtest.Periods = []*backtest.Backtest_Period{
		{
			StartDate: &pb.Date{Year: 2024, Month: 1, Day: 1},
			EndDate:   &pb.Date{Year: 2024, Month: 12, Day: 31},
			Assets: []*backtest.Backtest_Period_Asset{
				{
					Symbol:  "AAPL",
					Enabled: true,
				},
			},
		},
	}

	_, err := backtests.Create(ctx, test.backtest)
	test.Require().NoError(err)

	period := &backtest.Backtest_Period{
		StartDate: &pb.Date{Year: 2024, Month: 1, Day: 1},
		EndDate:   &pb.Date{Year: 2024, Month: 12, Day: 31},
	}
	asset := &backtest.Backtest_Period_Asset{
		Symbol:  "AAPL",
		Enabled: false,
	}
	err = backtests.UpdatePeriodAsset(ctx, test.backtest.Name, period, asset)
	test.Require().NoError(err)

	backtest, err := backtests.Get(ctx, test.backtest.Name)
	test.Require().NoError(err)
	test.Equal(test.backtest.Name, backtest.Name)
	test.Equal(period.StartDate, backtest.Periods[0].StartDate)
	test.Equal(period.EndDate, backtest.Periods[0].EndDate)
	test.Equal("AAPL", backtest.Periods[0].Assets[0].Symbol)
	test.False(backtest.Periods[0].Assets[0].Enabled)
}

func (test *BacktestTest) TestList() {
	ctx := context.Background()

	backtests := &repository.Backtest{Conn: test.conn}
	_, err := backtests.Create(ctx, test.backtest)
	test.Require().NoError(err)

	test.backtest.Name = "test_backtest_2"
	_, err = backtests.Create(ctx, test.backtest)
	test.Require().NoError(err)

	storedBacktests, err := backtests.List(ctx)
	test.Require().NoError(err)
	test.Len(storedBacktests, 2)
}

func (test *BacktestTest) TestDelete() {
	ctx := context.Background()

	backtests := &repository.Backtest{Conn: test.conn}
	_, err := backtests.Create(ctx, test.backtest)
	test.Require().NoError(err)

	err = backtests.Delete(ctx, test.backtest.Name)
	test.Require().NoError(err)

	storedBacktests, err := backtests.List(ctx)
	test.Require().NoError(err)
	test.Empty(storedBacktests)
}
