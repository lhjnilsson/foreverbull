package api

import (
	"context"
	"net/http/httptest"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/http"
	"github.com/lhjnilsson/foreverbull/strategy/internal/repository"
	"github.com/lhjnilsson/foreverbull/tests/helper"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
)

type ExecutionTest struct {
	suite.Suite

	router *gin.Engine
	log    *zap.Logger
}

func (test *ExecutionTest) SetupTest() {
	var err error

	test.log = zaptest.NewLogger(test.T())

	config := helper.TestingConfig(test.T(), &helper.Containers{
		Postgres: true,
	})
	conn, err := pgxpool.New(context.Background(), config.PostgresURI)
	test.NoError(err)
	err = repository.Recreate(context.Background(), conn)
	test.Nil(err)

	test.router = http.NewEngine()
	test.router.Use(
		func(ctx *gin.Context) {
			tx, err := conn.Begin(context.Background())
			if err != nil {
				ctx.AbortWithStatusJSON(500, http.APIError{Message: err.Error()})
				return
			}

			ctx.Set("log", test.log)
			ctx.Set("pgx_tx", tx)
			ctx.Next()
			err = tx.Commit(context.Background())
			if err != nil {
				ctx.AbortWithStatusJSON(500, http.APIError{Message: err.Error()})
				return
			}
		},
	)
}

func (test *ExecutionTest) addStrategy() {
	test.T().Helper()

	test.router.POST("/strategies", CreateStrategy)

	payload := `{"name": "test_strategy"}`
	req := httptest.NewRequest("POST", "/strategies", strings.NewReader(payload))
	w := httptest.NewRecorder()
	test.router.ServeHTTP(w, req)

	test.Equal(201, w.Code)
}

func (test *ExecutionTest) TestListExecutions() {
	test.router.GET("/executions", ListExecutions)

	req := httptest.NewRequest("GET", "/executions", nil)
	w := httptest.NewRecorder()
	test.router.ServeHTTP(w, req)

	test.Equal(200, w.Code)
	test.Equal("[]", w.Body.String())
}

func (test *ExecutionTest) TestCreateExecution() {
	test.addStrategy()

	test.router.POST("/executions", CreateExecution)

	req := httptest.NewRequest("POST", "/executions", nil)
	w := httptest.NewRecorder()
	test.router.ServeHTTP(w, req)

	test.Equal(201, w.Code)
}
