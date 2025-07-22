package dependency_test

import (
	"context"
	"fmt"
	"net"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/internal/stream"
	"github.com/lhjnilsson/foreverbull/internal/test_helper"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/repository"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/stream/dependency"
	"github.com/lhjnilsson/foreverbull/pkg/finance"
	"github.com/stretchr/testify/suite"
	"go.uber.org/fx"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type BacktestTest struct {
	suite.Suite

	pool *pgxpool.Pool
	app  *fx.App

	uut dependency.ETFScraper
}

func (test *BacktestTest) SetupTest() {
	test.uut = dependency.NewETFScaper()

}

func (test *BacktestTest) NoSetupTest() {
	var err error

	test_helper.SetupEnvironment(test.T(), &test_helper.Containers{
		Postgres: true,
		NATS:     true,
	})

	test.pool, err = pgxpool.New(context.Background(), environment.GetPostgresURL())
	test.Require().NoError(err)
	err = repository.Recreate(context.Background(), test.pool)
	test.Require().NoError(err)
	test.app = fx.New(
		fx.Provide(
			func() *pgxpool.Pool {
				return test.pool
			},
			func() *grpc.Server {
				return grpc.NewServer()
			},
		),
		fx.Invoke(
			func(lc fx.Lifecycle, gServer *grpc.Server) error {
				lc.Append(fx.Hook{
					OnStart: func(context.Context) error {
						listener, err := net.Listen("tcp", fmt.Sprintf(":%s", environment.GetGRPCPort()))
						if err != nil {
							return fmt.Errorf("failed to listen: %w", err)
						}
						go func() {
							if err := gServer.Serve(listener); err != nil {
								panic(err)
							}
						}()
						return nil
					},
					OnStop: func(context.Context) error {
						gServer.Stop()
						return nil
					},
				})
				return nil
			},
		),
		stream.Module,
		finance.Module,
	)
	test.Require().NoError(test.app.Start(context.TODO()))

	_, err = grpc.NewClient(environment.GetServerGRPCURL(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	test.Require().NoError(err, "failed to create grpc client")
}

func TestBacktest(t *testing.T) {
	suite.Run(t, new(BacktestTest))
}

func (test *BacktestTest) TestGetAssets() {
	assetMap, err := test.uut.GetAssets("iShares S&P 100", "2022", "2023")
	test.Require().NoError(err)
	test.Len(assetMap, 2)
	_, ok := assetMap["2022"]
	test.Require().True(ok)
	_, ok = assetMap["2023"]
	test.Require().True(ok)
}
