package api

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/backtest/api"
	"github.com/lhjnilsson/foreverbull/backtest/entity"
	"github.com/lhjnilsson/foreverbull/backtest/internal/repository"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/internal/http"
	"github.com/lhjnilsson/foreverbull/internal/stream"
	"github.com/lhjnilsson/foreverbull/tests/helper"
	"github.com/stretchr/testify/suite"
)

type IngestionTest struct {
	suite.Suite

	router *gin.Engine
	db     *pgxpool.Pool
	stream *stream.OrchestrationOutput
}

func (test *IngestionTest) SetupSuite() {
	helper.SetupEnvironment(test.T(), &helper.Containers{
		Postgres: true,
	})
}

func (test *IngestionTest) SetupTest() {
	var err error

	test.stream = &stream.OrchestrationOutput{}

	test.db, err = pgxpool.New(context.TODO(), environment.GetPostgresURL())
	test.Require().NoError(err)

	err = repository.Recreate(context.TODO(), test.db)
	test.Require().NoError(err)

	test.router = http.NewEngine()
	test.router.Use(
		func(ctx *gin.Context) {
			tx, err := test.db.Begin(context.Background())
			if err != nil {
				ctx.AbortWithStatusJSON(500, http.APIError{Message: err.Error()})
				return
			}

			ctx.Set(OrchestrationDependency, test.stream)
			ctx.Set(TXDependency, tx)
			ctx.Next()
			err = tx.Commit(context.Background())
			if err != nil {
				ctx.AbortWithStatusJSON(500, http.APIError{Message: err.Error()})
				return
			}
		},
	)
}

func (test *IngestionTest) SetupSubTest() {
	test.stream = &stream.OrchestrationOutput{}
}

func TestIngestion(t *testing.T) {
	suite.Run(t, new(IngestionTest))
}

func (test *IngestionTest) TestIngestion() {

	test.router.POST("/ingestion", CreateIngestion)
	test.router.GET("/ingestion", GetIngestion)

	test.Run("Create Ingestion", func() {
		body := api.CreateIngestionBody{
			Start:    "2021-01-01T00:00:00Z",
			End:      "2021-01-02T00:00:00Z",
			Calendar: "calendar",
			Symbols:  []string{"AAPL"},
		}
		bodyJSON, err := json.Marshal(body)
		test.Require().NoError(err)

		req := httptest.NewRequest("POST", "/ingestion", strings.NewReader(string(bodyJSON)))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		test.router.ServeHTTP(w, req)
		test.Require().Equal(201, w.Code)

		var ingestion entity.Ingestion
		err = json.Unmarshal(w.Body.Bytes(), &ingestion)
		test.Require().NoError(err)

		ingestions := repository.Ingestion{Conn: test.db}
		_, err = ingestions.Get(context.Background(), ingestion.Name)
		test.Require().NoError(err)
	})
	test.Run("Get Ingestion", func() {
		req := httptest.NewRequest("GET", "/ingestion", nil)
		w := httptest.NewRecorder()
		test.router.ServeHTTP(w, req)
		test.Require().Equal(200, w.Code)

		var ingestion *entity.Ingestion
		err := json.Unmarshal(w.Body.Bytes(), &ingestion)
		test.Require().NoError(err)
	})
}
