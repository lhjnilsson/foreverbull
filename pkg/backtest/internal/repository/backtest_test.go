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
	common_pb "github.com/lhjnilsson/foreverbull/pkg/pb"
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
	for index, start := range []*common_pb.Date{nil, {Year: 2024, Month: 0o1, Day: 0o1}} {
		test.Run(fmt.Sprintf("create backtest %d with start date", index), func() {
			backtests := &repository.Backtest{Conn: test.conn}
			backtest, err := backtests.Create(ctx,
				fmt.Sprintf("backtest_start_%d", index),
				start,
				nil,
				[]string{},
				nil,
			)
			if start == nil {
				test.Require().Error(err)
				test.Equal("failed to create backtest: start date is nil", err.Error())
			} else {
				test.Require().NoError(err)
				test.Equal(fmt.Sprintf("backtest_start_%d", index), backtest.Name)
				test.Equal(start, backtest.StartDate)
			}
		})
	}

	for index, end := range []*common_pb.Date{nil, {Year: 2024, Month: 0o1, Day: 0o1}} {
		test.Run(fmt.Sprintf("create backtest %d with end date", index), func() {
			backtests := &repository.Backtest{Conn: test.conn}
			backtest, err := backtests.Create(ctx,
				fmt.Sprintf("backtest_end_%d", index),
				&common_pb.Date{Year: 2024, Month: 0o1, Day: 0o1},
				end,
				[]string{},
				nil,
			)
			test.Require().NoError(err)
			test.Equal(fmt.Sprintf("backtest_end_%d", index), backtest.Name)
			test.Equal(end, backtest.EndDate)
		})
	}
	for index, benchmark := range []*string{nil, func() *string { s := "benchmark"; return &s }()} {
		test.Run(fmt.Sprintf("create backtest %d with benchmark %v", index, benchmark), func() {
			backtests := &repository.Backtest{Conn: test.conn}
			backtest, err := backtests.Create(ctx,
				fmt.Sprintf("backtest_bench_%d", index),
				&common_pb.Date{Year: 2024, Month: 0o1, Day: 0o1},
				nil,
				[]string{},
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
		test.Equal(pgx.ErrNoRows, err)
	})
	test.Run("found", func() {
		backtests := &repository.Backtest{Conn: test.conn}
		_, err := backtests.Create(ctx, "backtest", &common_pb.Date{Year: 2024, Month: 0o1, Day: 0o1}, &common_pb.Date{Year: 2024, Month: 0o1, Day: 0o1}, []string{}, nil)
		test.Require().NoError(err)

		backtest, err := backtests.Get(ctx, "backtest")
		test.Require().NoError(err)
		test.Equal("backtest", backtest.Name)
	})
}

func (test *BacktestTest) TestUpdate() {
	ctx := context.Background()

	backtests := &repository.Backtest{Conn: test.conn}
	_, err := backtests.Create(ctx, "backtest", &common_pb.Date{Year: 2024, Month: 0o1, Day: 0o1}, &common_pb.Date{Year: 2024, Month: 0o1, Day: 0o1}, []string{}, nil)
	test.Require().NoError(err)

	_, err = backtests.Update(ctx, "backtest", &common_pb.Date{Year: 2024, Month: 0o1, Day: 0o1}, &common_pb.Date{Year: 2024, Month: 0o1, Day: 0o1}, []string{"AAPL"}, nil)
	test.Require().NoError(err)

	backtest, err := backtests.Get(ctx, "backtest")
	test.Require().NoError(err)
	test.Equal("backtest", backtest.Name)
	test.Equal([]string{"AAPL"}, backtest.Symbols)
	test.Nil(backtest.Benchmark)
}

func (test *BacktestTest) TestList() {
	ctx := context.Background()

	backtests := &repository.Backtest{Conn: test.conn}
	_, err := backtests.Create(ctx, "backtest1", &common_pb.Date{Year: 2024, Month: 0o1, Day: 0o1}, &common_pb.Date{Year: 2024, Month: 0o1, Day: 0o1}, []string{}, nil)
	test.Require().NoError(err)

	_, err = backtests.Create(ctx, "backtest2", &common_pb.Date{Year: 2024, Month: 0o1, Day: 0o1}, nil, []string{}, nil)
	test.Require().NoError(err)

	storedBacktests, err := backtests.List(ctx)
	test.Require().NoError(err)
	test.Len(storedBacktests, 2)
}

func (test *BacktestTest) TestDelete() {
	ctx := context.Background()

	backtests := &repository.Backtest{Conn: test.conn}
	_, err := backtests.Create(ctx, "backtest", &common_pb.Date{Year: 2024, Month: 0o1, Day: 0o1}, &common_pb.Date{Year: 2024, Month: 0o1, Day: 0o1}, []string{}, nil)
	test.Require().NoError(err)

	err = backtests.Delete(ctx, "backtest")
	test.Require().NoError(err)

	storedBacktests, err := backtests.List(ctx)
	test.Require().NoError(err)
	test.Empty(storedBacktests)
}
