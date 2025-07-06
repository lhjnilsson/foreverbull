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
	financeServicePB "github.com/lhjnilsson/foreverbull/pkg/pb/finance/service"
	"github.com/stretchr/testify/suite"
	"go.uber.org/fx"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type BacktestTest struct {
	suite.Suite

	pool *pgxpool.Pool
	app  *fx.App

	uut dependency.ETFLookup
}

func (test *BacktestTest) SetupTest() {
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

	conn, err := grpc.NewClient(environment.GetServerGRPCURL(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	test.Require().NoError(err, "failed to create grpc client")
	financeClient := financeServicePB.NewMarketdataClient(conn)

	test.uut = dependency.ETFLookup{
		FinanceAPI: financeClient,
	}
}

func TestBacktest(t *testing.T) {
	suite.Run(t, new(BacktestTest))
}

func (test *BacktestTest) TestDemo() {
	err := test.uut.Get("https://www.ishares.com/uk/individual/en/products/253741/ishares-nasdaq-100-ucits-etf/1506575576011.ajax", "2022")
	test.Require().NoError(err)
}
