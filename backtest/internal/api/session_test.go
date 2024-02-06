package api

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/backtest/entity"
	"github.com/lhjnilsson/foreverbull/backtest/internal/repository"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/internal/http"
	"github.com/lhjnilsson/foreverbull/internal/stream"
	"github.com/lhjnilsson/foreverbull/tests/helper"
	"github.com/stretchr/testify/suite"
)

type SessionTest struct {
	suite.Suite

	db     *pgxpool.Pool
	router *gin.Engine
	stream *stream.PendingOrchestration
}

func (test *SessionTest) SetupTest() {
	var err error

	test.stream = &stream.PendingOrchestration{}

	helper.SetupEnvironment(test.T(), &helper.Containers{
		Postgres: true,
	})
	test.db, err = pgxpool.New(context.Background(), environment.GetPostgresURL())
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

func (test *SessionTest) SetupSubTest() {
	test.stream = &stream.PendingOrchestration{}
}

func TestSession(t *testing.T) {
	suite.Run(t, new(SessionTest))
}

func (test *SessionTest) TestListSessions() {
	test.router.GET("/sessions", ListSessions)

	req := httptest.NewRequest("GET", "/sessions", nil)
	w := httptest.NewRecorder()
	test.router.ServeHTTP(w, req)

	test.Equal(200, w.Code)
	test.Equal("[]", w.Body.String())
}

func (test *SessionTest) TestCreateSession() {
	AddBacktest(test.T(), test.db, "test_backtest")

	test.router.POST("/sessions", CreateSession)

	type TestCase struct {
		name         string
		payload      string
		expectedCode int
		manual       bool
	}
	testCases := []TestCase{
		{
			name:         "no backtest",
			payload:      `{}`,
			expectedCode: 400,
		},
		{
			name:         "no executions",
			payload:      `{"backtest": "test_backtest"}`,
			expectedCode: 400,
		},
		{
			name:         "empty execution",
			payload:      `{"backtest": "test_backtest", "executions": []}`,
			expectedCode: 400,
		},
		{
			name:         "default execution",
			payload:      `{"backtest": "test_backtest", "executions": [{}]}`,
			expectedCode: 201,
		},
		{
			name:         "default execution with start",
			payload:      `{"backtest": "test_backtest", "executions": [{"start": "2020-01-01T00:00:00Z"}]}`,
			expectedCode: 201,
		},
		{
			name:         "default execution with end",
			payload:      `{"backtest": "test_backtest", "executions": [{"end": "2020-06-01T00:00:00Z"}]}`,
			expectedCode: 201,
		},
		{
			name:         "default execution with symbols",
			payload:      `{"backtest": "test_backtest", "executions": [{"symbols": ["AAPL"]}]}`,
			expectedCode: 201,
		},
		{
			name:         "default execution with benchmark",
			payload:      `{"backtest": "test_backtest", "executions": [{"benchmark": "AAPL"}]}`,
			expectedCode: 201,
		},
		{
			name:         "manual true",
			payload:      `{"backtest": "test_backtest", "manual": true}`,
			expectedCode: 201,
			manual:       true,
		},
	}
	for _, tc := range testCases {
		test.Run(tc.name, func() {
			req := httptest.NewRequest("POST", "/sessions", strings.NewReader(tc.payload))
			w := httptest.NewRecorder()
			test.router.ServeHTTP(w, req)

			test.Equal(tc.expectedCode, w.Code)
			if tc.expectedCode == 201 {
				test.True(test.stream.Contains("run backtest session"))
			} else {
				test.False(test.stream.Contains("run backtest session"))
			}
			body := entity.Session{}
			err := json.Unmarshal(w.Body.Bytes(), &body)
			test.Nil(err)
			test.Equal(tc.manual, body.Manual)
		})
	}
}

func (test *SessionTest) TestCreateSessionStartEnd() {
	backtest := AddBacktest(test.T(), test.db, "test_backtest")

	test.router.POST("/sessions", CreateSession)

	type TestCase struct {
		name         string
		Start        time.Time
		End          time.Time
		expectedCode int
	}
	testCases := []TestCase{
		{
			name:         "start before backtest",
			Start:        backtest.Start.Add(-24 * time.Hour),
			End:          backtest.End,
			expectedCode: 400,
		},
		{
			name:         "end after backtest",
			Start:        backtest.Start,
			End:          backtest.End.Add(24 * time.Hour),
			expectedCode: 400,
		},
		{
			name:         "start after end",
			Start:        backtest.End,
			End:          backtest.Start,
			expectedCode: 400,
		},
	}
	for _, tc := range testCases {
		test.Run(tc.name, func() {
			payload := `{"backtest": "test_backtest", "executions": [{"start": "` + tc.Start.Format("2006-01-02") + `", "end": "` + tc.End.Format("2006-01-02") + `"}]}`
			req := httptest.NewRequest("POST", "/sessions", strings.NewReader(payload))
			w := httptest.NewRecorder()
			test.router.ServeHTTP(w, req)

			test.Equal(tc.expectedCode, w.Code)
		})
	}
}

func (test *SessionTest) TestGetSession() {
	AddBacktest(test.T(), test.db, "test_backtest")

	test.router.POST("/sessions", CreateSession)
	test.router.GET("/sessions/:id", GetSession)

	req := httptest.NewRequest("GET", "/sessions/123", nil)
	w := httptest.NewRecorder()
	test.router.ServeHTTP(w, req)

	test.Equal(404, w.Code)

	payload := `{"backtest": "test_backtest", "executions": [{}]}`
	req = httptest.NewRequest("POST", "/sessions", strings.NewReader(payload))
	w = httptest.NewRecorder()
	test.router.ServeHTTP(w, req)

	test.Equal(201, w.Code)
	session := entity.Session{}
	err := json.Unmarshal(w.Body.Bytes(), &session)
	test.Nil(err)

	req = httptest.NewRequest("GET", "/sessions/"+session.ID, nil)
	w = httptest.NewRecorder()
	test.router.ServeHTTP(w, req)

	test.Equal(200, w.Code)
}
