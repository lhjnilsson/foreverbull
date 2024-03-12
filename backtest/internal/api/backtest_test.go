package api

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/internal/stream"

	"github.com/gin-gonic/gin"
	"github.com/lhjnilsson/foreverbull/backtest/internal/repository"
	"github.com/lhjnilsson/foreverbull/internal/http"
	"github.com/lhjnilsson/foreverbull/tests/helper"
	"github.com/stretchr/testify/suite"
)

type BacktestTest struct {
	suite.Suite

	router *gin.Engine
	stream *stream.OrchestrationOutput
}

func (test *BacktestTest) SetupTest() {
	test.stream = &stream.OrchestrationOutput{}

	helper.SetupEnvironment(test.T(), &helper.Containers{
		Postgres: true,
	})
	pool, err := pgxpool.New(context.Background(), environment.GetPostgresURL())
	test.Require().NoError(err)

	err = repository.Recreate(context.TODO(), pool)
	test.Require().NoError(err)

	test.router = http.NewEngine()
	test.router.Use(
		func(ctx *gin.Context) {
			tx, err := pool.Begin(context.Background())
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

func (test *BacktestTest) SetupSubTest() {
	test.stream = &stream.OrchestrationOutput{}
}

func TestBacktest(t *testing.T) {
	suite.Run(t, new(BacktestTest))
}

func (test *BacktestTest) TestListBacktests() {
	test.router.GET("/backtests", ListBacktests)

	req := httptest.NewRequest("GET", "/backtests", nil)
	w := httptest.NewRecorder()
	test.router.ServeHTTP(w, req)

	test.Equal(200, w.Code)
	test.Equal("[]", w.Body.String())
}

func (test *BacktestTest) TestCreateBacktest() {
	test.router.POST("/backtests", CreateBacktest)

	type TestCase struct {
		name         string
		payload      string
		expectedCode int
	}
	testCases := []TestCase{
		{
			name: "no name",
			payload: `{"calendar": "XNYS",
			"start": "2020-01-01T00:00:00Z", "end": "2020-01-01T00:00:00Z", "symbols": ["AAPL"]}`,
			expectedCode: 400,
		},
		{
			name: "no worker_service",
			payload: `{"name": "no worker_service", "calendar": "XNYS",
			"start": "2020-01-01T00:00:00Z", "end": "2020-01-01T00:00:00Z", "symbols": ["AAPL"]}`,
			expectedCode: 201,
		},
		{
			name: "no calendar",
			payload: `{"name": "no calendar", "service": "worker",
			"start": "2020-01-01T00:00:00Z", "end": "2020-01-01T00:00:00Z", "symbols": ["AAPL"]}`,
			expectedCode: 400,
		},
		{
			name: "no benchmark",
			payload: `{"name": "no benchmark", "service": "worker", 
			"calendar": "XNYS", "start": "2020-01-01T00:00:00Z", "end": "2020-01-01T00:00:00Z", 
			"symbols": ["AAPL"]}`,
			expectedCode: 201,
		},
		{
			name: "no symbols",
			payload: `{"name": "no symbols", "service": "worker",
			"symbols": [], "calendar": "XNYS", "start": "2020-01-01T00:00:00Z", "end": "2020-01-01T00:00:00Z"}`,
			expectedCode: 400,
		},
	}
	for _, testCase := range testCases {
		test.Run(testCase.name, func() {
			req := httptest.NewRequest("POST", "/backtests", strings.NewReader(testCase.payload))
			w := httptest.NewRecorder()
			test.router.ServeHTTP(w, req)
			test.Equal(testCase.expectedCode, w.Code, testCase.name)
			if testCase.expectedCode == 201 {
				test.True(test.stream.Contains("ingest backtest"))
			} else {
				test.False(test.stream.Contains("ingest backtest"))
			}

		})
	}
}

func (test *BacktestTest) TestCreateBacktestTimeFormats() {
	test.router.POST("/backtests", CreateBacktest)

	type TestCase struct {
		name         string
		Start        string
		End          string
		ExpectedCode int
	}
	testCases := []TestCase{
		{
			name:         "RFC3339",
			Start:        "2020-01-01T00:00:00Z",
			End:          "2020-01-01T00:00:00Z",
			ExpectedCode: 201,
		},
		{
			name:         "RFC3339Nano",
			Start:        "2020-01-01T00:00:00.000000000Z",
			End:          "2020-01-01T00:00:00.000000000Z",
			ExpectedCode: 201,
		},
		{
			name:         "DateOnly",
			Start:        "2020-01-01",
			End:          "2020-01-01",
			ExpectedCode: 201,
		},
		{
			name:         "invalid start",
			Start:        "2020-01-01T00:00:00",
			End:          "2020-01-01T00:00:00Z",
			ExpectedCode: 400,
		},
		{
			name:         "invalid end",
			Start:        "2020-01-01T00:00:00Z",
			End:          "2020-01-01T00:00:00",
			ExpectedCode: 400,
		},
	}
	for _, testCase := range testCases {
		test.Run(testCase.name, func() {
			payload := `{"name": "` + testCase.name + `", "calendar": "XNYS", 
			"start": "` + testCase.Start + `", "end": "` + testCase.End + `", "symbols": ["AAPL"]}`
			req := httptest.NewRequest("POST", "/backtests", strings.NewReader(payload))
			w := httptest.NewRecorder()
			test.router.ServeHTTP(w, req)
			test.Equal(testCase.ExpectedCode, w.Code, testCase.name)
			if testCase.ExpectedCode == 201 {
				test.True(test.stream.Contains("ingest backtest"))
			} else {
				test.False(test.stream.Contains("ingest backtest"))
			}
		})
	}
}

func (test *BacktestTest) TestGetBacktest() {
	test.router.POST("/backtests", CreateBacktest)
	test.router.GET("/backtests/:name", GetBacktest)

	req := httptest.NewRequest("GET", "/backtests/test_backtest", nil)
	w := httptest.NewRecorder()
	test.router.ServeHTTP(w, req)

	test.Equal(404, w.Code)

	payload := `{"name": "test_backtest", "calendar": "XNYS", 
	"start": "2020-01-01T00:00:00Z", "end": "2020-01-01T00:00:00Z", "symbols": ["AAPL"]}`
	req = httptest.NewRequest("POST", "/backtests", strings.NewReader(payload))
	w = httptest.NewRecorder()
	test.router.ServeHTTP(w, req)

	test.Equal(201, w.Code)

	req = httptest.NewRequest("GET", "/backtests/test_backtest", nil)
	w = httptest.NewRecorder()
	test.router.ServeHTTP(w, req)

	test.Equal(200, w.Code)
}

func (test *BacktestTest) TestUpdateBacktest() {
	test.router.POST("/backtests", CreateBacktest)
	test.router.PUT("/backtests/:name", UpdateBacktest)

	payload := `{"name": "test_backtest", "calendar": "XNYS", 
	"start": "2020-01-01T00:00:00Z", "end": "2020-01-01T00:00:00Z", "symbols": ["AAPL"], "benchmark": "SPY"}`
	req := httptest.NewRequest("POST", "/backtests", strings.NewReader(payload))
	w := httptest.NewRecorder()
	test.router.ServeHTTP(w, req)
	test.Equal(201, w.Code)

	// Recreate to make sure its created after we update
	test.True(test.stream.Contains("ingest backtest"))
	test.stream = &stream.OrchestrationOutput{}

	payload = `{"name": "test_backtest", "calendar": "XNYS", 
	"start": "2020-01-01T00:00:00Z", "end": "2020-01-01T00:00:00Z", "symbols": ["AAPL"], "benchmark": "SPY"}`
	req = httptest.NewRequest("PUT", "/backtests/test_backtest", strings.NewReader(payload))
	w = httptest.NewRecorder()
	test.router.ServeHTTP(w, req)
	test.Equal(200, w.Code)
	test.True(test.stream.Contains("ingest backtest"))
}

func (test *BacktestTest) TestDeleteBacktest() {
	test.router.POST("/backtests", CreateBacktest)
	test.router.DELETE("/backtests/:name", DeleteBacktest)

	payload := `{"name": "test_backtest", "calendar": "XNYS", 
	"start": "2020-01-01T00:00:00Z", "end": "2020-01-01T00:00:00Z", "symbols": ["AAPL"]}`
	req := httptest.NewRequest("POST", "/backtests", strings.NewReader(payload))
	w := httptest.NewRecorder()
	test.router.ServeHTTP(w, req)

	test.Equal(201, w.Code)

	req = httptest.NewRequest("DELETE", "/backtests/test_backtest", nil)
	w = httptest.NewRecorder()
	test.router.ServeHTTP(w, req)

	test.Equal(204, w.Code)
}
