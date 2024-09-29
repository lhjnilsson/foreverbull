package service

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	h "github.com/lhjnilsson/foreverbull/internal/http"
	"github.com/lhjnilsson/foreverbull/internal/stream"
	"github.com/lhjnilsson/foreverbull/internal/test_helper"
	"github.com/lhjnilsson/foreverbull/pkg/finance"
	"github.com/lhjnilsson/foreverbull/pkg/service/internal/repository"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/suite"
	"go.uber.org/fx"
)

type ServiceModuleTest struct {
	suite.Suite

	app *fx.App
}

func TestModuleService(t *testing.T) {
	_, found := os.LookupEnv("IMAGES")
	if !found {
		t.Skip("images not set")
	}
	suite.Run(t, new(ServiceModuleTest))
}

func (test *ServiceModuleTest) SetupTest() {
	test_helper.SetupEnvironment(test.T(), &test_helper.Containers{
		Postgres: true,
		NATS:     true,
		Loki:     true,
	})
	pool, err := pgxpool.New(context.Background(), environment.GetPostgresURL())
	test.NoError(err)
	err = repository.Recreate(context.Background(), pool)
	test.NoError(err)
	test.app = fx.New(
		fx.Provide(
			func() (*nats.Conn, nats.JetStreamContext, error) {
				return stream.New()
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
		finance.Module,
		Module,
	)
	test.Require().NoError(test.app.Start(context.TODO()))
}

func (test *ServiceModuleTest) TearDownTest() {
	test_helper.WaitTillContainersAreRemoved(test.T(), environment.GetDockerNetworkName(), time.Second*20)
	test.NoError(test.app.Stop(context.Background()))
}
