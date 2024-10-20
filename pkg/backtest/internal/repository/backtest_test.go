package repository_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	common_pb "github.com/lhjnilsson/foreverbull/internal/pb"
	"github.com/lhjnilsson/foreverbull/internal/test_helper"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/repository"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/pb"
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

	for i, end := range []*common_pb.Date{nil, {Year: 2024, Month: 0o1, Day: 0o1}} {
		backtests := &repository.Backtest{Conn: test.conn}
		backtest, err := backtests.Create(ctx,
			fmt.Sprintf("backtest_%d", i),
			&common_pb.Date{Year: 2024, Month: 0o1, Day: 0o1},
			end,
			[]string{},
			nil,
		)
		test.Require().NoError(err)
		test.Equal(fmt.Sprintf("backtest_%d", i), backtest.Name)
		test.Len(backtest.Statuses, 1)
		test.Equal(pb.Backtest_Status_CREATED.String(), backtest.Statuses[0].Status.String())
		test.Equal(end, backtest.EndDate)
	}
}

func (test *BacktestTest) TestGet() {
	ctx := context.Background()

	backtests := &repository.Backtest{Conn: test.conn}
	_, err := backtests.Create(ctx, "backtest", &common_pb.Date{Year: 2024, Month: 0o1, Day: 0o1}, &common_pb.Date{Year: 2024, Month: 0o1, Day: 0o1}, []string{}, nil)
	test.Require().NoError(err)

	backtest, err := backtests.Get(ctx, "backtest")
	test.Require().NoError(err)
	test.Equal("backtest", backtest.Name)
	test.Len(backtest.Statuses, 1)
	test.Equal(pb.Backtest_Status_CREATED.String(), backtest.Statuses[0].Status.String())
}

func (test *BacktestTest) TestGetUniverse() {
	ctx := context.Background()

	backtests := &repository.Backtest{Conn: test.conn}
	test.Run("without stored data", func() {
		start, end, symbols, err := backtests.GetUniverse(ctx)
		test.Require().Error(err)
		test.Nil(start)
		test.Nil(end)
		test.Nil(symbols)
	})
	test.Run("with stored data", func() {
		_, err := backtests.Create(ctx, "nasdaq", &common_pb.Date{Year: 2024, Month: 0o1, Day: 0o1}, &common_pb.Date{Year: 2024, Month: 0o6, Day: 0o1}, []string{"AAPL", "MSFT"}, nil)
		test.Require().NoError(err)
		_, err = backtests.Create(ctx, "nyse", &common_pb.Date{Year: 2024, Month: 0o1, Day: 0o1}, &common_pb.Date{Year: 2024, Month: 0o4, Day: 0o1}, []string{"IBM", "GE"}, nil)
		test.Require().NoError(err)

		start, end, symbols, err := backtests.GetUniverse(ctx)
		test.Require().NoError(err)
		test.Equal(&common_pb.Date{Year: 2024, Month: 0o1, Day: 0o1}, start)
		test.Equal(&common_pb.Date{Year: 2024, Month: 0o6, Day: 0o1}, end)
		test.ElementsMatch([]string{"AAPL", "MSFT", "IBM", "GE"}, symbols)
	})
	test.Run("with None as end", func() {
		_, err := backtests.Create(ctx, "none", &common_pb.Date{Year: 2024, Month: 0o1, Day: 0o1}, nil, []string{"AAPL", "MSFT"}, nil)
		test.Require().NoError(err)

		start, end, symbols, err := backtests.GetUniverse(ctx)
		expectedEnd := common_pb.GoTimeToDate(time.Now())

		test.Require().NoError(err)
		test.Equal(&common_pb.Date{Year: 2024, Month: 0o1, Day: 0o1}, start)
		test.Equal(expectedEnd, end)
		test.ElementsMatch([]string{"AAPL", "MSFT", "IBM", "GE"}, symbols)
	})
	test.Run("with benchmark", func() {
		benchmark := "^DJI"
		_, err := backtests.Create(ctx, "bench", &common_pb.Date{Year: 2024, Month: 0o1, Day: 0o1}, nil, []string{"AAPL", "MSFT"}, &benchmark)
		test.Require().NoError(err)

		start, end, symbols, err := backtests.GetUniverse(ctx)
		expectedEnd := common_pb.GoTimeToDate(time.Now())

		test.Require().NoError(err)
		test.Equal(&common_pb.Date{Year: 2024, Month: 0o1, Day: 0o1}, start)
		test.Equal(expectedEnd, end)
		test.ElementsMatch([]string{"AAPL", "MSFT", "IBM", "GE", "^DJI"}, symbols)
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

func (test *BacktestTest) TestUpdateStatus() {
	ctx := context.Background()

	backtests := &repository.Backtest{Conn: test.conn}
	_, err := backtests.Create(ctx, "backtest", &common_pb.Date{Year: 2024, Month: 0o1, Day: 0o1}, &common_pb.Date{Year: 2024, Month: 0o1, Day: 0o1}, []string{}, nil)
	test.Require().NoError(err)

	err = backtests.UpdateStatus(ctx, "backtest", pb.Backtest_Status_ERROR, errors.New("test"))
	test.Require().NoError(err)

	backtest, err := backtests.Get(ctx, "backtest")
	test.Require().NoError(err)
	test.Equal("backtest", backtest.Name)
	test.Len(backtest.Statuses, 2)
	test.Equal(pb.Backtest_Status_ERROR.String(), backtest.Statuses[0].Status.String())
	test.NotNil(backtest.Statuses[0].OccurredAt)
	test.Equal("test", *backtest.Statuses[0].Error)
	test.Equal(pb.Backtest_Status_CREATED.String(), backtest.Statuses[1].Status.String())
	test.NotNil(backtest.Statuses[1].OccurredAt)
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
