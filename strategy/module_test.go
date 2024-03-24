package strategy

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/finance"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	h "github.com/lhjnilsson/foreverbull/internal/http"
	"github.com/lhjnilsson/foreverbull/internal/stream"
	"github.com/lhjnilsson/foreverbull/service"
	"github.com/lhjnilsson/foreverbull/strategy/entity"
	"github.com/lhjnilsson/foreverbull/strategy/internal/repository"
	"github.com/lhjnilsson/foreverbull/tests/helper"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/suite"
	"go.uber.org/fx"
)

type ModuleTests struct {
	suite.Suite
	app *fx.App
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
	payload := fmt.Sprintf(`{"name": "test-strategy", "min_days": 30, "symbols": ["AAPL", "MSFT", "GOOG"], "service": "%s"}`, os.Getenv("WORKER_IMAGE"))
	rsp := helper.Request(test.T(), "POST", "/strategy/api/strategies", payload)
	test.Equal(201, rsp.StatusCode)

	type ExecutionResponse struct {
		ID       string
		Statuses []struct {
			Status entity.ExecutionStatusType
		}
	}
	rsp = helper.Request(test.T(), "POST", "/strategy/api/executions", fmt.Sprintf(`{"strategy": "%s"}`, "test-strategy"))
	test.Equal(201, rsp.StatusCode)
	response := &ExecutionResponse{}
	err := json.NewDecoder(rsp.Body).Decode(response)
	test.NoError(err)
	condition := func() (bool, error) {
		rsp := helper.Request(test.T(), "GET", "/strategy/api/executions/"+response.ID, "")
		if rsp.StatusCode != 200 {
			return false, nil
		}
		response := &ExecutionResponse{}
		err := json.NewDecoder(rsp.Body).Decode(response)
		if err != nil {
			return false, fmt.Errorf("failed to decode response: %s", err.Error())
		}
		if len(response.Statuses) == 0 {
			return false, nil
		}
		return response.Statuses[0].Status == entity.ExecutionStatusCompleted, nil
	}
	test.NoError(helper.WaitUntilCondition(test.T(), condition, time.Second*20))
}
