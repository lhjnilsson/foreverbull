package repository

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	common_pb "github.com/lhjnilsson/foreverbull/internal/pb"
	"github.com/lhjnilsson/foreverbull/internal/test_helper"
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

	for i, end := range []*common_pb.Date{nil, {Year: 2024, Month: 01, Day: 01}} {
		db := &Backtest{Conn: test.conn}
		backtest, err := db.Create(ctx,
			fmt.Sprintf("backtest_%d", i),
			&common_pb.Date{Year: 2024, Month: 01, Day: 01},
			end,
			[]string{},
			nil,
		)
		test.NoError(err)
		test.Equal(fmt.Sprintf("backtest_%d", i), backtest.Name)
		test.Len(backtest.Statuses, 1)
		test.Equal(pb.Backtest_Status_CREATED.String(), backtest.Statuses[0].Status.String())
		test.Equal(end, backtest.EndDate)
	}
}

func (test *BacktestTest) TestGet() {
	ctx := context.Background()

	db := &Backtest{Conn: test.conn}
	_, err := db.Create(ctx, "backtest", &common_pb.Date{Year: 2024, Month: 01, Day: 01}, &common_pb.Date{Year: 2024, Month: 01, Day: 01}, []string{}, nil)
	test.NoError(err)

	backtest, err := db.Get(ctx, "backtest")
	test.NoError(err)
	test.Equal("backtest", backtest.Name)
	test.Len(backtest.Statuses, 1)
	test.Equal(pb.Backtest_Status_CREATED.String(), backtest.Statuses[0].Status.String())
}

func (test *BacktestTest) TestUpdate() {
	ctx := context.Background()

	db := &Backtest{Conn: test.conn}
	_, err := db.Create(ctx, "backtest", &common_pb.Date{Year: 2024, Month: 01, Day: 01}, &common_pb.Date{Year: 2024, Month: 01, Day: 01}, []string{}, nil)
	test.NoError(err)

	_, err = db.Update(ctx, "backtest", &common_pb.Date{Year: 2024, Month: 01, Day: 01}, &common_pb.Date{Year: 2024, Month: 01, Day: 01}, []string{"AAPL"}, nil)
	test.NoError(err)

	backtest, err := db.Get(ctx, "backtest")
	test.NoError(err)
	test.Equal("backtest", backtest.Name)
	// TODO: FIX, Github looses nanoseconds
	// -(time.Time) 2023-10-19 19:53:22.382093481 +0000 UTC
	// +(time.Time) 2023-10-19 19:53:22.382093 +0000 UTC
	//test.Equal(start, *backtest.Start)
	//test.Equal(end, *backtest.End)
	test.Equal([]string{"AAPL"}, backtest.Symbols)
	test.Nil(backtest.Benchmark)
}

func (test *BacktestTest) TestUpdateStatus() {
	ctx := context.Background()

	db := &Backtest{Conn: test.conn}
	_, err := db.Create(ctx, "backtest", &common_pb.Date{Year: 2024, Month: 01, Day: 01}, &common_pb.Date{Year: 2024, Month: 01, Day: 01}, []string{}, nil)
	test.NoError(err)

	err = db.UpdateStatus(ctx, "backtest", pb.Backtest_Status_ERROR, errors.New("test"))
	test.NoError(err)

	backtest, err := db.Get(ctx, "backtest")
	test.NoError(err)
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

	db := &Backtest{Conn: test.conn}
	_, err := db.Create(ctx, "backtest1", &common_pb.Date{Year: 2024, Month: 01, Day: 01}, &common_pb.Date{Year: 2024, Month: 01, Day: 01}, []string{}, nil)
	test.NoError(err)

	_, err = db.Create(ctx, "backtest2", &common_pb.Date{Year: 2024, Month: 01, Day: 01}, nil, []string{}, nil)
	test.NoError(err)

	backtests, err := db.List(ctx)
	test.NoError(err)
	test.Len(backtests, 2)
}

func (test *BacktestTest) TestDelete() {
	ctx := context.Background()

	db := &Backtest{Conn: test.conn}
	_, err := db.Create(ctx, "backtest", &common_pb.Date{Year: 2024, Month: 01, Day: 01}, &common_pb.Date{Year: 2024, Month: 01, Day: 01}, []string{}, nil)
	test.NoError(err)

	err = db.Delete(ctx, "backtest")
	test.NoError(err)

	backtests, err := db.List(ctx)
	test.NoError(err)
	test.Len(backtests, 0)
}
