//go:build integration

package stream_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/container"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/internal/grpc"
	"github.com/lhjnilsson/foreverbull/internal/postgres"
	"github.com/lhjnilsson/foreverbull/internal/storage"
	st "github.com/lhjnilsson/foreverbull/internal/stream"
	"github.com/lhjnilsson/foreverbull/internal/test_helper"
	"github.com/lhjnilsson/foreverbull/pkg/backtest"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/repository"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/stream"
	"github.com/lhjnilsson/foreverbull/pkg/finance"
	"github.com/lhjnilsson/foreverbull/pkg/pb"
	backtestpb "github.com/lhjnilsson/foreverbull/pkg/pb/backtest"
	"github.com/lhjnilsson/foreverbull/pkg/service"
	"github.com/lhjnilsson/foreverbull/pkg/strategy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"go.uber.org/fx"
)

func TestCreateBacktest(t *testing.T) {
	o, err := stream.CreateBacktestFromETF("sp100_backtest", "iShares S&P 100", "2022-01-01", "2023-01-01")
	assert.NoError(t, err)
	assert.Equal(t, int32(1), o.Steps[0].Num)
	assert.Equal(t, int32(2), o.Steps[1].Num)
	assert.Equal(t, int32(3), o.Steps[2].Num)
}

type BacktestTest struct {
	suite.Suite

	stream st.Stream
	db     postgres.Query

	app *fx.App
}

func (test *BacktestTest) SetupSuite() {
	test_helper.SetupEnvironment(test.T(), &test_helper.Containers{
		Postgres: true,
		NATS:     true,
		Minio:    true,
		Zipline:  true,
	})

	var err error
	pool, err := pgxpool.New(context.Background(), environment.GetPostgresURL())
	test.Require().NoError(err)
	test.db = pool
	test.app = fx.New(
		fx.Provide(
			func() *pgxpool.Pool {
				return pool
			},
			func() (storage.Storage, error) {
				return storage.NewMinioStorage(context.Background())
			},
			container.NewEngine,
		),
		fx.Invoke(),
		grpc.Module,
		backtest.Module,
		finance.Module,
		service.Module,
		strategy.Module,
		st.Module,
	)
	test.stream, err = st.NewNATSStream()
	test.Require().NoError(err)
}

func (test *BacktestTest) SetupTest() {
	test.Require().NoError(test.app.Start(context.Background()), "failed to start app")
}

func (test *BacktestTest) TearDownTest() {
	test.NoError(test.app.Stop(context.Background()), "failed to stop app")
}

func TestBacktest(t *testing.T) {
	suite.Run(t, new(BacktestTest))
}

func (test *BacktestTest) TestCreateBacktestFromETF() {
	o, err := stream.CreateBacktestFromETF("sp100_backtest", "iShares S&P 100", "2022-01-01", "2023-01-01")
	test.Require().NoError(err)
	test.Require().NotNil(o)

	b := backtestpb.Backtest{
		Name: "sp100_backtest",
		Periods: []*backtestpb.Backtest_Period{
			{
				StartDate: &pb.Date{Year: 2022, Month: 1, Day: 1},
				EndDate:   &pb.Date{Year: 2022, Month: 12, Day: 31},
			},
			{
				StartDate: &pb.Date{Year: 2023, Month: 1, Day: 1},
				EndDate:   &pb.Date{Year: 2023, Month: 12, Day: 31},
			},
		},
	}

	backtests := repository.Backtest{Conn: test.db}
	_, err = backtests.Create(context.TODO(), &b)
	test.Require().NoError(err)

	steps := test.stream.RunOrchestration(context.TODO(), o)
	for step := range steps {
		test.T().Logf("Step: %v", step)
	}

	minio, err := storage.NewMinioStorage(context.TODO())
	test.Require().NoError(err)
	object, err := minio.GetObject(context.TODO(), storage.IngestionsBucket, "sp100_backtest")
	test.Require().NoError(err)
	test.Equal("sp100_backtest", object.Name)
}

func (test *BacktestTest) TestCreateBacktestFromETFBadETF() {
	o, err := stream.CreateBacktestFromETF("sp100_backtest", "bad", "2022-01-01", "2023-01-01")
	test.Require().NoError(err)
	test.Require().NotNil(o)

	b := backtestpb.Backtest{
		Name: "sp100_backtest",
		Periods: []*backtestpb.Backtest_Period{
			{
				StartDate: &pb.Date{Year: 2022, Month: 1, Day: 1},
				EndDate:   &pb.Date{Year: 2022, Month: 12, Day: 31},
			},
			{
				StartDate: &pb.Date{Year: 2023, Month: 1, Day: 1},
				EndDate:   &pb.Date{Year: 2023, Month: 12, Day: 31},
			},
		},
	}

	backtests := repository.Backtest{Conn: test.db}
	_, err = backtests.Create(context.TODO(), &b)
	test.Require().NoError(err)

	steps := test.stream.RunOrchestration(context.TODO(), o)
	for step := range steps {
		test.T().Logf("Step: %v", step)
	}

	minio, err := storage.NewMinioStorage(context.TODO())
	test.Require().NoError(err)
	object, err := minio.GetObject(context.TODO(), storage.IngestionsBucket, "sp100_backtest")
	test.Require().NoError(err)
	test.Equal("sp100_backtest", object.Name)
}
