package api

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/internal/http"
	"github.com/lhjnilsson/foreverbull/internal/stream"
	"github.com/lhjnilsson/foreverbull/internal/test_helper"
	"github.com/lhjnilsson/foreverbull/pkg/strategy/internal/repository"
	"github.com/stretchr/testify/suite"
)

type ExecutionTest struct {
	suite.Suite

	db     *pgxpool.Pool
	router *gin.Engine
	stream *stream.OrchestrationOutput
}

func (test *ExecutionTest) SetupSuite() {
	test_helper.SetupEnvironment(test.T(), &test_helper.Containers{
		Postgres: true,
	})
}

func (test *ExecutionTest) SetupTest() {
	var err error

	test.stream = &stream.OrchestrationOutput{}

	test.db, err = pgxpool.New(context.Background(), environment.GetPostgresURL())
	test.Require().NoError(err)
	err = repository.Recreate(context.Background(), test.db)
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

func TestExecution(t *testing.T) {
	suite.Run(t, new(ExecutionTest))
}

func (test *ExecutionTest) TestListExecutions() {
	test.router.GET("/executions", ListExecutions)

	req := httptest.NewRequest("GET", "/executions?strategy=demo", nil)
	w := httptest.NewRecorder()
	test.router.ServeHTTP(w, req)

	test.Require().Equal(200, w.Code)
	test.Equal("[]", w.Body.String())
}

func (test *ExecutionTest) TestCreateExecution() {
	strategies := repository.Strategy{Conn: test.db}
	_, err := strategies.Create(context.Background(), "test-strategy", []string{"symbol"}, 0, "worker-service")
	test.Require().NoError(err)

	test.router.POST("/executions", CreateExecution)

	payload := `{"strategy": "test-strategy"}`

	req := httptest.NewRequest("POST", "/executions", strings.NewReader(payload))
	w := httptest.NewRecorder()
	test.router.ServeHTTP(w, req)

	test.Require().Equal(201, w.Code)
}

func (test *ExecutionTest) TestGetExecutionNotFound() {
	test.router.GET("/executions/:id", GetExecution)

	req := httptest.NewRequest("GET", "/executions/1", nil)
	w := httptest.NewRecorder()
	test.router.ServeHTTP(w, req)

	test.Require().Equal(404, w.Code)
}
