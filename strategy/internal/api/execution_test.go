package api

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/internal/http"
	"github.com/lhjnilsson/foreverbull/strategy/internal/repository"
	"github.com/lhjnilsson/foreverbull/tests/helper"
	"github.com/stretchr/testify/suite"
)

type ExecutionTest struct {
	suite.Suite

	router *gin.Engine
}

func (test *ExecutionTest) SetupSuite() {
	helper.SetupEnvironment(test.T(), &helper.Containers{
		Postgres: true,
	})
}

func (test *ExecutionTest) SetupTest() {
	conn, err := pgxpool.New(context.Background(), environment.GetPostgresURL())
	test.Require().NoError(err)
	err = repository.Recreate(context.Background(), conn)
	test.Require().NoError(err)

	test.router = http.NewEngine()
	test.router.Use(
		func(ctx *gin.Context) {
			tx, err := conn.Begin(context.Background())
			if err != nil {
				ctx.AbortWithStatusJSON(500, http.APIError{Message: err.Error()})
				return
			}

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

func (test *ExecutionTest) TestGetExecutionNotFound() {
	test.router.GET("/executions/:id", GetExecution)

	req := httptest.NewRequest("GET", "/executions/1", nil)
	w := httptest.NewRecorder()
	test.router.ServeHTTP(w, req)

	test.Require().Equal(404, w.Code)
}
