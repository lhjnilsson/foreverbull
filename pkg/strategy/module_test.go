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
	"github.com/lhjnilsson/foreverbull/internal/environment"
	h "github.com/lhjnilsson/foreverbull/internal/http"
	"github.com/lhjnilsson/foreverbull/internal/stream"
	"github.com/lhjnilsson/foreverbull/internal/test_helper"
	"github.com/lhjnilsson/foreverbull/pkg/finance"
	financeEntity "github.com/lhjnilsson/foreverbull/pkg/finance/entity"
	"github.com/lhjnilsson/foreverbull/pkg/service"
	"github.com/lhjnilsson/foreverbull/pkg/strategy/entity"
	"github.com/lhjnilsson/foreverbull/pkg/strategy/internal/repository"
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

}

func (test *ModuleTests) SetupTest() {
	test_helper.SetupEnvironment(test.T(), &test_helper.Containers{
		Postgres: true,
		NATS:     true,
		Loki:     true,
	})

	pool, err := pgxpool.New(context.TODO(), environment.GetPostgresURL())
	test.Require().NoError(err)
	err = repository.Recreate(context.Background(), pool)
	test.Require().NoError(err)

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
		service.Module,
		finance.Module,
		Module,
	)
	test.Require().NoError(test.app.Start(context.Background()))
}

func (test *ModuleTests) TearDownTest() {
	test_helper.WaitTillContainersAreRemoved(test.T(), environment.GetDockerNetworkName(), time.Second*20)
	test.Require().NoError(test.app.Stop(context.Background()))
}

func (test *ModuleTests) TestRunStrategyExecution() {
	payload := fmt.Sprintf(`{"name": "test-strategy", "min_days": 30, "symbols": ["AAPL", "MSFT", "GOOG"], "service": "%s"}`, os.Getenv("WORKER_IMAGE"))
	rsp := test_helper.Request(test.T(), "POST", "/strategy/api/strategies", payload)
	test.Equal(201, rsp.StatusCode)

	type ExecutionResponse struct {
		ID       string
		Statuses []struct {
			Status entity.ExecutionStatusType
		}
		StartPortfolio financeEntity.Portfolio `json:"start_portfolio"`
		PlacedOrders   []financeEntity.Order   `json:"placed_orders"`
	}
	rsp = test_helper.Request(test.T(), "POST", "/strategy/api/executions", fmt.Sprintf(`{"strategy": "%s"}`, "test-strategy"))
	test.Equal(201, rsp.StatusCode)
	response := &ExecutionResponse{}
	err := json.NewDecoder(rsp.Body).Decode(response)
	test.NoError(err)
	condition := func() (bool, error) {
		rsp := test_helper.Request(test.T(), "GET", "/strategy/api/executions/"+response.ID, "")
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
	test.NoError(test_helper.WaitUntilCondition(test.T(), condition, time.Second*20))

	rsp = test_helper.Request(test.T(), "GET", "/strategy/api/executions/"+response.ID, "")
	test.Equal(200, rsp.StatusCode)
	response = &ExecutionResponse{}
	err = json.NewDecoder(rsp.Body).Decode(response)
	test.NoError(err)
	test.Equal(entity.ExecutionStatusCompleted, response.Statuses[0].Status)
	test.NotEmpty(response.StartPortfolio)
	test.NotEmpty(response.PlacedOrders)
}
