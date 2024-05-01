package api

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/internal/http"
	"github.com/lhjnilsson/foreverbull/service/entity"
	"github.com/lhjnilsson/foreverbull/service/internal/repository"
	"github.com/lhjnilsson/foreverbull/tests/helper"
	mockStream "github.com/lhjnilsson/foreverbull/tests/mocks/internal_/stream"
	"github.com/stretchr/testify/suite"
)

type InstanceTest struct {
	suite.Suite

	router *gin.Engine
	stream *mockStream.Stream

	conn *pgxpool.Pool
}

func (test *InstanceTest) SetupTest() {
	var err error
	test.stream = new(mockStream.Stream)

	helper.SetupEnvironment(test.T(), &helper.Containers{
		Postgres: true,
	})
	test.conn, err = pgxpool.New(context.Background(), environment.GetPostgresURL())
	test.Require().NoError(err)
	err = repository.Recreate(context.Background(), test.conn)
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

func TestInstance(t *testing.T) {
	suite.Run(t, new(InstanceTest))
}

func (test *InstanceTest) TestListInstances() {
	test.router.GET("/instances", ListInstances)

	type TestCase struct {
		Image             string
		ExpectedInstances int
	}

	testCases := []TestCase{
		{
			Image:             "image1",
			ExpectedInstances: 2,
		},
		{
			Image:             "image2",
			ExpectedInstances: 1,
		},
		{
			Image:             "image3",
			ExpectedInstances: 0,
		},
	}

	for _, testCase := range testCases {
		image := AddService(test.T(), test.conn, testCase.Image)
		for i := 0; i < testCase.ExpectedInstances; i++ {
			AddInstance(test.T(), test.conn, &image)
		}
	}

	for _, testCase := range testCases {
		test.Run(testCase.Image, func() {
			req := httptest.NewRequest("GET", "/instances?image="+testCase.Image, nil)
			w := httptest.NewRecorder()
			test.router.ServeHTTP(w, req)

			test.Equal(200, w.Code)
			instances := []entity.Instance{}
			err := json.Unmarshal(w.Body.Bytes(), &instances)
			test.Nil(err)
			test.Len(instances, testCase.ExpectedInstances)
		})
	}

	test.Run("all", func() {
		req := httptest.NewRequest("GET", "/instances", nil)
		w := httptest.NewRecorder()
		test.router.ServeHTTP(w, req)
		test.Equal(200, w.Code)
		instances := []entity.Instance{}
		err := json.Unmarshal(w.Body.Bytes(), &instances)
		test.Nil(err)
		test.Len(instances, 3)
	})

	test.Run("not_stored", func() {
		req := httptest.NewRequest("GET", "/instances?image=not_stored", nil)
		w := httptest.NewRecorder()
		test.router.ServeHTTP(w, req)
		test.Equal(200, w.Code)
		instances := []entity.Instance{}
		err := json.Unmarshal(w.Body.Bytes(), &instances)
		test.Nil(err)
		test.Len(instances, 0)
	})
}

func (test *InstanceTest) TestGetInstance() {
	test.router.GET("/instances/:instanceID", GetInstance)

	req := httptest.NewRequest("GET", "/instances/instance123", nil)
	w := httptest.NewRecorder()
	test.router.ServeHTTP(w, req)

	test.Equal(404, w.Code)

	serviceName := AddService(test.T(), test.conn, "test_service")
	instanceID := AddInstance(test.T(), test.conn, &serviceName)

	req = httptest.NewRequest("GET", "/instances/"+instanceID, nil)
	w = httptest.NewRecorder()
	test.router.ServeHTTP(w, req)

	test.Equal(200, w.Code)
}

func (test *InstanceTest) TestPatchInstance() {
	test.router.PATCH("/instances/:instanceID", PatchInstance)
	test.router.GET("/instances/:instanceID", GetInstance)

	serviceName := AddService(test.T(), test.conn, "test_service")
	instanceID := AddInstance(test.T(), test.conn, &serviceName)

	payload := `{"host": "127.0.0.1", "port": 1337}`
	req := httptest.NewRequest("PATCH", "/instances/"+instanceID, strings.NewReader(payload))
	w := httptest.NewRecorder()
	test.router.ServeHTTP(w, req)

	test.Equal(200, w.Code)

	req = httptest.NewRequest("GET", "/instances/"+instanceID, nil)
	w = httptest.NewRecorder()
	test.router.ServeHTTP(w, req)

	instance := &entity.Instance{}
	err := json.Unmarshal(w.Body.Bytes(), instance)
	test.Nil(err)
	test.Equal("127.0.0.1", *instance.Host)
	test.Equal(1337, *instance.Port)
}
