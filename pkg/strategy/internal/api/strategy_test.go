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
	"github.com/lhjnilsson/foreverbull/internal/test_helper"
	"github.com/lhjnilsson/foreverbull/pkg/strategy/internal/repository"
	"github.com/stretchr/testify/suite"
)

type StrategyTest struct {
	suite.Suite

	router *gin.Engine
}

func (test *StrategyTest) SetupSuite() {
	test_helper.SetupEnvironment(test.T(), &test_helper.Containers{
		Postgres: true,
	})
}

func (test *StrategyTest) SetupTest() {
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

func TestStrategy(t *testing.T) {
	suite.Run(t, new(StrategyTest))
}

func (test *StrategyTest) TestListStrategies() {
	test.router.GET("/strategies", ListStrategies)

	req := httptest.NewRequest("GET", "/strategies", nil)
	w := httptest.NewRecorder()
	test.router.ServeHTTP(w, req)

	test.Require().Equal(200, w.Code)
	test.Equal("[]", w.Body.String())
}

func (test *StrategyTest) TestCreateStrategy() {
	test.router.POST("/strategies", CreateStrategy)

	type TestCase struct {
		name         string
		payload      string
		expectedCode int
	}
	testCases := []TestCase{
		{
			name:         "valid",
			payload:      `{"name":"test","symbols":["AAPL"],"min_days":1,"service":"test"}`,
			expectedCode: 201,
		},
		{
			name:         "no name",
			payload:      `{"symbols":["AAPL"],"min_days":1,"service":"test"}`,
			expectedCode: 400,
		},
	}
	for _, tc := range testCases {
		req := httptest.NewRequest("POST", "/strategies", strings.NewReader(tc.payload))
		w := httptest.NewRecorder()
		test.router.ServeHTTP(w, req)

		test.Equal(tc.expectedCode, w.Code)
	}
}

func (test *StrategyTest) TestGetStrategyNotFound() {
	test.router.GET("/strategies/:name", GetStrategy)

	req := httptest.NewRequest("GET", "/strategies/abc123", nil)
	w := httptest.NewRecorder()
	test.router.ServeHTTP(w, req)

	test.Require().Equal(404, w.Code)
}

func (test *StrategyTest) TestGetStrategy() {
	test.router.POST("/strategies", CreateStrategy)
	test.router.GET("/strategies/:name", GetStrategy)

	req := httptest.NewRequest("POST", "/strategies", strings.NewReader(`{"name":"test","symbols":["AAPL"],"min_days":1,"service":"test"}`))
	w := httptest.NewRecorder()
	test.router.ServeHTTP(w, req)

	test.Require().Equal(201, w.Code)

	req2 := httptest.NewRequest("GET", "/strategies/test", nil)
	w2 := httptest.NewRecorder()
	test.router.ServeHTTP(w2, req2)

	test.Require().Equal(200, w2.Code)
}

func (test *StrategyTest) TestDeleteStrategy() {
	test.router.POST("/strategies", CreateStrategy)
	test.router.DELETE("/strategies/:name", DeleteStrategy)

	req := httptest.NewRequest("POST", "/strategies", strings.NewReader(`{"name":"test","symbols":["AAPL"],"min_days":1,"service":"test"}`))
	w := httptest.NewRecorder()
	test.router.ServeHTTP(w, req)

	test.Require().Equal(201, w.Code)

	req2 := httptest.NewRequest("DELETE", "/strategies/test", nil)
	w2 := httptest.NewRecorder()
	test.router.ServeHTTP(w2, req2)

	test.Require().Equal(204, w2.Code)
}
