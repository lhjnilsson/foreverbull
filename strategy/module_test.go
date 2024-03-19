package strategy

import (
	"context"
	"os"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/finance"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	h "github.com/lhjnilsson/foreverbull/internal/http"
	"github.com/lhjnilsson/foreverbull/internal/stream"
	"github.com/lhjnilsson/foreverbull/service"
	"github.com/lhjnilsson/foreverbull/strategy/internal/repository"
	"github.com/lhjnilsson/foreverbull/tests/helper"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/suite"
	"go.uber.org/fx"
)

type ModuleTests struct {
	suite.Suite
	app *fx.App

	strategyName string
}

func TestStrategyModule(t *testing.T) {
	workerImage := os.Getenv("WORKER_IMAGE")
	if workerImage == "" {
		t.Skip("worker image not set")
	}
	suite.Run(t, new(ModuleTests))
}

func (test *ModuleTests) SetupSuite() {
	helper.SetupEnvironment(test.T(), &helper.Containers{
		Postgres: true,
		NATS:     true,
		Loki:     true,
	})
}

func (test *ModuleTests) SetupTest() {
	pool, err := pgxpool.New(context.TODO(), environment.GetPostgresURL())
	test.Require().NoError(err)
	err = repository.Recreate(context.Background(), pool)
	test.Require().NoError(err)

	test.app = fx.New(
		fx.Provide(
			func() (nats.JetStreamContext, error) {
				return stream.NewJetstream()
			},
			func() *pgxpool.Pool {
				return pool
			},
			func() *gin.Engine {
				return h.NewEngine()
			},
		),
		fx.Invoke(
			h.NewLifeCycleRouter,
		),
		stream.OrchestrationLifecycle,
		service.Module,
		finance.Module,
		Module,
	)
	test.Require().NoError(test.app.Start(context.Background()))
}

func (test *ModuleTests) TearDownTest() {
	test.Require().NoError(test.app.Stop(context.Background()))
}

func (test *ModuleTests) TestRunStrategyExecution() {
}
