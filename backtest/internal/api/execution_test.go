package api

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/backtest/internal/repository"
	"github.com/lhjnilsson/foreverbull/internal/http"
	"github.com/lhjnilsson/foreverbull/tests/helper"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
)

type ExecutionTest struct {
	suite.Suite

	db     *pgxpool.Pool
	router *gin.Engine
	log    *zap.Logger
}

func (test *ExecutionTest) SetupTest() {
	var err error
	test.log = zaptest.NewLogger(test.T())

	config := helper.TestingConfig(test.T(), &helper.Containers{
		Postgres: true,
	})
	test.db, err = pgxpool.New(context.Background(), config.PostgresURI)
	test.NoError(err)

	err = repository.Recreate(context.TODO(), test.db)
	test.Nil(err)

	test.router = http.NewEngine()
	test.router.Use(
		func(ctx *gin.Context) {
			tx, err := test.db.Begin(context.Background())
			if err != nil {
				ctx.AbortWithStatusJSON(500, http.APIError{Message: err.Error()})
				return
			}

			ctx.Set(LoggingDependency, test.log)
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

	req := httptest.NewRequest("GET", "/executions", nil)
	w := httptest.NewRecorder()
	test.router.ServeHTTP(w, req)

	test.Equal(200, w.Code)
	test.Equal("[]", w.Body.String())
}

func (test *ExecutionTest) TestListExecutionsBySession() {
	test.router.GET("/executions", ListExecutions)

	req := httptest.NewRequest("GET", "/executions?session=123", nil)
	w := httptest.NewRecorder()
	test.router.ServeHTTP(w, req)

	test.Equal(200, w.Code)
	test.Equal("[]", w.Body.String())
}

func (test *ExecutionTest) TestGetExecution() {
	backtest := AddBacktest(test.T(), test.db, "test_backtest")
	session := AddSession(test.T(), test.db, backtest.Name)

	test.router.GET("/executions/:id", GetExecution)

	req := httptest.NewRequest("GET", "/executions/123", nil)
	w := httptest.NewRecorder()
	test.router.ServeHTTP(w, req)

	test.Equal(404, w.Code)

	execution := AddExecution(test.T(), test.db, session.ID)

	req = httptest.NewRequest("GET", "/executions/"+execution.ID, nil)
	w = httptest.NewRecorder()
	test.router.ServeHTTP(w, req)

	test.Equal(200, w.Code)
}

func (test *ExecutionTest) TestGetExecutionPeriods() {
	backtest := AddBacktest(test.T(), test.db, "test_backtest")
	session := AddSession(test.T(), test.db, backtest.Name)
	execution := AddExecution(test.T(), test.db, session.ID)

	test.router.GET("/executions/:id/periods", GetExecutionPeriods)

	req := httptest.NewRequest("GET", "/executions/"+execution.ID+"/periods", nil)
	w := httptest.NewRecorder()
	test.router.ServeHTTP(w, req)

	test.Equal(200, w.Code)
}

func (test *ExecutionTest) TestGetExecutionMetrics() {
	backtest := AddBacktest(test.T(), test.db, "test_backtest")
	session := AddSession(test.T(), test.db, backtest.Name)
	execution := AddExecution(test.T(), test.db, session.ID)

	test.router.GET("/executions/:id/metrics", GetExecutionPeriodMetrics)

	req := httptest.NewRequest("GET", "/executions/"+execution.ID+"/metrics", nil)
	w := httptest.NewRecorder()
	test.router.ServeHTTP(w, req)

	test.Equal(200, w.Code)
}
