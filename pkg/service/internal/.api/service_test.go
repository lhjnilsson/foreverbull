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
	"github.com/lhjnilsson/foreverbull/pkg/service/container"
	"github.com/lhjnilsson/foreverbull/pkg/service/internal/repository"
	"github.com/stretchr/testify/suite"
)

type ServiceTest struct {
	suite.Suite

	router    *gin.Engine
	stream    *stream.OrchestrationOutput
	container *container.MockContainer

	conn *pgxpool.Pool
}

func (test *ServiceTest) SetupTest() {
	var err error

	test.stream = &stream.OrchestrationOutput{}
	test.container = new(container.MockContainer)

	test_helper.SetupEnvironment(test.T(), &test_helper.Containers{
		Postgres: true,
	})
	test.conn, err = pgxpool.New(context.Background(), environment.GetPostgresURL())
	test.Require().NoError(err)
	err = repository.Recreate(context.TODO(), test.conn)
	test.Require().NoError(err)

	test.router = http.NewEngine()
	test.router.Use(
		func(ctx *gin.Context) {
			tx, err := test.conn.Begin(context.Background())
			if err != nil {
				ctx.AbortWithStatusJSON(500, http.APIError{Message: err.Error()})
				return
			}

			ctx.Set(OrchestrationDependency, test.stream)
			ctx.Set(ContainerDependency, test.container)
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

func (test *ServiceTest) SetupSubTest() {
	test.stream = &stream.OrchestrationOutput{}
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
			name:         "missing image",
			payload:      `{}`,
			expectedCode: 400,
		},
		{
			name:         "valid",
			payload:      `{"image": "image"}`,
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
	test.router.GET("/services/*image", GetService)
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

	test.router.DELETE("/services/*image", DeleteService)

	req := httptest.NewRequest("DELETE", "/services/"+serviceName, nil)
	w := httptest.NewRecorder()
	test.router.ServeHTTP(w, req)

	test.Equal(204, w.Code)
}
