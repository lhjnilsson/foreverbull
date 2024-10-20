package finance_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	internal_pb "github.com/lhjnilsson/foreverbull/internal/pb"
	"github.com/lhjnilsson/foreverbull/internal/stream"
	"github.com/lhjnilsson/foreverbull/internal/test_helper"
	"github.com/lhjnilsson/foreverbull/pkg/finance"
	"github.com/lhjnilsson/foreverbull/pkg/finance/internal/repository"
	fs "github.com/lhjnilsson/foreverbull/pkg/finance/stream"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/suite"
	"go.uber.org/fx"
)

type FinanceModuleTest struct {
	suite.Suite

	app *fx.App

	pool *pgxpool.Pool
}

func TestFinanceModule(t *testing.T) {
	apiKey := os.Getenv("ALPACA_MARKETS_API_KEY")
	if apiKey == "" {
		t.Skip("ALPACA_MARKETS_API_KEY not set")
	}

	suite.Run(t, new(FinanceModuleTest))
}

func (test *FinanceModuleTest) SetupTest() {
	test_helper.SetupEnvironment(test.T(), &test_helper.Containers{
		Postgres: true,
		NATS:     true,
	})

	pool, err := pgxpool.New(context.Background(), environment.GetPostgresURL())
	test.Require().NoError(err)
	err = repository.Recreate(context.Background(), test.pool)
	test.Require().NoError(err)
	test.app = fx.New(
		fx.Provide(
			func() (*nats.Conn, nats.JetStreamContext, error) {
				return stream.New()
			},
			func() *pgxpool.Pool {
				return pool
			},
		),
		stream.OrchestrationLifecycle,
		finance.Module,
	)
	test.Require().NoError(test.app.Start(context.TODO()))
}

func (test *FinanceModuleTest) TearDownTest() {
	test.NoError(test.app.Stop(context.Background()))
}

func (test *FinanceModuleTest) TestIngestCommand() {
	_, st, err := stream.New()
	test.Require().NoError(err)
	stream, err := stream.NewNATSStream(st, "finance_test", stream.NewDependencyContainer(), test.pool)
	test.Require().NoError(err)

	endDate := "2020-02-01"
	command, err := fs.NewIngestCommand([]string{"AAPL"}, "2020-01-01", &endDate)
	test.Require().NoError(err)
	test.NoError(stream.Publish(context.Background(), command))

	ohlcExists := func() (bool, error) {
		ohlc := repository.OHLC{Conn: test.pool}

		return ohlc.Exists(context.Background(), []string{"AAPL"}, &internal_pb.Date{Year: 2020, Month: 1, Day: 1}, &internal_pb.Date{Year: 2020, Month: 2, Day: 1})
	}
	test.NoError(test_helper.WaitUntilCondition(test.T(), ohlcExists, 10*time.Second))
}
