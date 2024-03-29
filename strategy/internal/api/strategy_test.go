package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/internal/http"
	"github.com/lhjnilsson/foreverbull/strategy/internal/repository"
	"github.com/lhjnilsson/foreverbull/tests/helper"
	"github.com/stretchr/testify/suite"
)

type StrategyTest struct {
	suite.Suite

	router *gin.Engine
}

func (test *StrategyTest) SetupTest() {
	var err error
	helper.SetupEnvironment(test.T(), &helper.Containers{
		Postgres: true,
	})
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

	test.Equal(200, w.Code)
	test.Equal("[]", w.Body.String())
}

func (test *StrategyTest) TestCreateStrategy() {
	test.router.POST("/strategies", CreateStrategy)

	payload := `{"name": "test_strategy"}`
	req := httptest.NewRequest("POST", "/strategies", strings.NewReader(payload))
	w := httptest.NewRecorder()
	test.router.ServeHTTP(w, req)

	test.Equal(201, w.Code)
}

func (test *StrategyTest) TestGetStrategy() {
	test.router.POST("/strategies", CreateStrategy)
	test.router.GET("/strategies/:name", GetStrategy)

	payload := `{"name": "test_strategy"}`
	req := httptest.NewRequest("POST", "/strategies", strings.NewReader(payload))
	w := httptest.NewRecorder()
	test.router.ServeHTTP(w, req)

	test.Equal(201, w.Code)
	strategy := map[string]interface{}{}
	err := json.Unmarshal(w.Body.Bytes(), &strategy)
	test.Nil(err)

	req = httptest.NewRequest("GET", fmt.Sprintf("/strategies/%s", strategy["name"]), nil)
	w = httptest.NewRecorder()
	test.router.ServeHTTP(w, req)

	test.Equal(200, w.Code)
}

func (test *StrategyTest) TestPatchStrategy() {
	test.router.POST("/strategies", CreateStrategy)
	test.router.PATCH("/strategies/:name", PatchStrategy)

	payload := `{"name": "test_strategy"}`
	req := httptest.NewRequest("POST", "/strategies", strings.NewReader(payload))
	w := httptest.NewRecorder()
	test.router.ServeHTTP(w, req)

	test.Equal(201, w.Code)
	strategy := map[string]interface{}{}
	err := json.Unmarshal(w.Body.Bytes(), &strategy)
	test.Nil(err)

	payload = `{"backtest": "test_backtest"}`
	req = httptest.NewRequest("PATCH", fmt.Sprintf("/strategies/%s", strategy["name"]), strings.NewReader(payload))
	w = httptest.NewRecorder()
	test.router.ServeHTTP(w, req)

	test.Equal(200, w.Code)
}

func (test *StrategyTest) TestDeleteStrategy() {
	test.router.POST("/strategies", CreateStrategy)
	test.router.DELETE("/strategies/:name", DeleteStrategy)

	payload := `{"name": "test_strategy"}`
	req := httptest.NewRequest("POST", "/strategies", strings.NewReader(payload))
	w := httptest.NewRecorder()
	test.router.ServeHTTP(w, req)

	test.Equal(201, w.Code)
	strategy := map[string]interface{}{}
	err := json.Unmarshal(w.Body.Bytes(), &strategy)
	test.Nil(err)

	req = httptest.NewRequest("DELETE", fmt.Sprintf("/strategies/%s", strategy["name"]), nil)
	w = httptest.NewRecorder()
	test.router.ServeHTTP(w, req)

	test.Equal(204, w.Code)
}
