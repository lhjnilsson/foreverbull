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
	"github.com/lhjnilsson/foreverbull/service/internal/repository"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"

	"github.com/lhjnilsson/foreverbull/tests/helper"
	mockContainer "github.com/lhjnilsson/foreverbull/tests/mocks/service/container"
)

type ServiceTest struct {
	suite.Suite

	router    *gin.Engine
	log       *zap.Logger
	stream    *stream.PendingOrchestration
	container *mockContainer.Container

	conn *pgxpool.Pool
}

func (test *ServiceTest) SetupTest() {
	var err error

	test.log = zap.NewNop()
	test.stream = &stream.PendingOrchestration{}
	test.container = new(mockContainer.Container)

	helper.SetupEnvironment(test.T(), &helper.Containers{
		Postgres: true,
	})
	test.conn, err = pgxpool.New(context.Background(), environment.GetPostgresURL())
	test.NoError(err)
	err = repository.Recreate(context.TODO(), test.conn)
	test.Nil(err)

	test.router = http.NewEngine()
	test.router.Use(
		func(ctx *gin.Context) {
			tx, err := test.conn.Begin(context.Background())
			if err != nil {
				ctx.AbortWithStatusJSON(500, http.APIError{Message: err.Error()})
				return
			}

			ctx.Set(LoggingDependency, test.log)
			ctx.Set(OrchestrationDependency, test.stream)
			ctx.Set(ContainerDependency, test.container)
			ctx.Set(TXDependency, tx)
			ctx.Next()
			err = tx.Commit(context.Background())
			if err != nil {
				test.log.Error("Failed to commit transaction", zap.Error(err))
				ctx.AbortWithStatusJSON(500, http.APIError{Message: err.Error()})
				return
			}
		},
	)
}

func (test *ServiceTest) SetupSubTest() {
	test.stream = &stream.PendingOrchestration{}
}

func TestService(t *testing.T) {
	suite.Run(t, new(ServiceTest))
}

func (test *ServiceTest) TestListServices() {
	test.router.GET("/services", ListServices)

	req := httptest.NewRequest("GET", "/services", nil)
	w := httptest.NewRecorder()
	test.router.ServeHTTP(w, req)

	test.Equal(200, w.Code)
	test.Equal("[]", w.Body.String())
}

func (test *ServiceTest) TestCreateService() {
	test.router.POST("/services", CreateService)

	type TestCase struct {
		name         string
		payload      string
		expectedCode int
	}
	testCases := []TestCase{
		{
			name:         "missing name",
			payload:      `{"image": "image"}`,
			expectedCode: 400,
		},
		{
			name:         "name too short",
			payload:      `{"name": "s", "image": "image"}`,
			expectedCode: 400,
		},
		{
			name:         "missing image",
			payload:      `{"name": "service"}`,
			expectedCode: 400,
		},
		{
			name:         "image too short",
			payload:      `{"name": "service", "image": "i"}`,
			expectedCode: 400,
		},
		{
			name:         "valid",
			payload:      `{"name": "service", "image": "image"}`,
			expectedCode: 201,
		},
	}
	for _, testCase := range testCases {
		test.Run(testCase.name, func() {
			req := httptest.NewRequest("POST", "/services", strings.NewReader(testCase.payload))
			w := httptest.NewRecorder()
			test.router.ServeHTTP(w, req)
			test.Equal(testCase.expectedCode, w.Code)
			if testCase.expectedCode == 201 {
				test.True(test.stream.Contains("service interview"))
			} else {
				test.False(test.stream.Contains("service interview"))
			}
		})
	}
}

func (test *ServiceTest) TestGetService() {
	test.router.GET("/services/:name", GetService)
	req := httptest.NewRequest("GET", "/services/service123", nil)
	w := httptest.NewRecorder()
	test.router.ServeHTTP(w, req)

	test.Equal(404, w.Code)

	serviceName := AddService(test.T(), test.conn, "test_service")

	req = httptest.NewRequest("GET", "/services/"+serviceName, nil)
	w = httptest.NewRecorder()
	test.router.ServeHTTP(w, req)

	test.Equal(200, w.Code)
}

func (test *ServiceTest) TestDeleteService() {
	serviceName := AddService(test.T(), test.conn, "test_service")

	test.router.DELETE("/services/:name", DeleteService)

	req := httptest.NewRequest("DELETE", "/services/"+serviceName, nil)
	w := httptest.NewRecorder()
	test.router.ServeHTTP(w, req)

	test.Equal(204, w.Code)
}
