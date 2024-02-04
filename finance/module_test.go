package finance

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/finance/internal/repository"
	fs "github.com/lhjnilsson/foreverbull/finance/stream"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	h "github.com/lhjnilsson/foreverbull/internal/http"
	"github.com/lhjnilsson/foreverbull/internal/stream"
	"github.com/lhjnilsson/foreverbull/tests/helper"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/suite"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
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
	helper.SetupEnvironment(test.T(), &helper.Containers{
		Postgres: true,
		NATS:     true,
	})
	log := zaptest.NewLogger(test.T(), zaptest.Level(zap.DebugLevel))
	st, err := stream.NewJetstream(environment.GetNATSURL())
	test.Require().NoError(err)
	test.pool, err = pgxpool.New(context.Background(), environment.GetPostgresURL())
	test.Require().NoError(err)
	err = repository.Recreate(context.Background(), test.pool)
	test.Require().NoError(err)
	g := h.NewEngine()
	test.app = fx.New(
		fx.Provide(
			func() *zap.Logger {
				return log
			},
			func() nats.JetStreamContext {
				return st
			},
			func() *pgxpool.Pool {
				return test.pool
			},
			func() *gin.Engine {
				return g
			},
		),
		fx.Invoke(
			h.NewLifeCycleRouter,
		),
		stream.OrchestrationLifecycle,
		Module,
	)
	test.Require().NoError(test.app.Start(context.TODO()))
}

func (test *FinanceModuleTest) TearDownTest() {
	test.NoError(test.app.Stop(context.Background()))
}

func (test *FinanceModuleTest) TestIngestCommand() {
	st, err := stream.NewJetstream(environment.GetNATSURL())
	test.NoError(err)
	stream, err := stream.NewNATSStream(st, "finance_test", zaptest.NewLogger(test.T()), stream.NewDependencyContainer(), test.pool)
	test.NoError(err)

	command, err := fs.NewIngestCommand([]string{"AAPL", "MSFT"}, time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(2020, 2, 1, 0, 0, 0, 0, time.UTC))
	test.NoError(err)
	command, err = stream.CreateMessage(context.Background(), command)
	test.NoError(err)
	test.NoError(stream.Publish(context.Background(), command))

	ohlcExists := func() (bool, error) {
		ohlc := repository.OHLC{Conn: test.pool}
		return ohlc.Exists(context.Background(), []string{"AAPL"}, time.Date(2020, 1, 2, 0, 0, 0, 0, time.UTC), time.Date(2020, 1, 31, 0, 0, 0, 0, time.UTC))
	}
	test.NoError(helper.WaitUntilCondition(test.T(), ohlcExists, 10*time.Second))
}
