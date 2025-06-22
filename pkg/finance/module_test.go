package finance_test

import (
	"context"
	"fmt"
	"net"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/internal/stream"
	"github.com/lhjnilsson/foreverbull/internal/test_helper"
	"github.com/lhjnilsson/foreverbull/pkg/finance"
	"github.com/lhjnilsson/foreverbull/pkg/finance/internal/repository"
	"github.com/stretchr/testify/suite"
	"go.uber.org/fx"
	"google.golang.org/grpc"
)

type FinanceModuleTest struct {
	suite.Suite

	app *fx.App

	pool *pgxpool.Pool
}

func TestFinanceModule(t *testing.T) {
	suite.Run(t, new(FinanceModuleTest))
}

func (test *FinanceModuleTest) SetupTest() {
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
}

func (test *FinanceModuleTest) TearDownTest() {
	test.NoError(test.app.Stop(context.Background()))
}

func (test *FinanceModuleTest) TestBlank() {}
