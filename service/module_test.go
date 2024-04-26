package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	docker "github.com/docker/docker/client"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	h "github.com/lhjnilsson/foreverbull/internal/http"
	"github.com/lhjnilsson/foreverbull/internal/stream"
	"github.com/lhjnilsson/foreverbull/service/api"
	"github.com/lhjnilsson/foreverbull/service/internal/repository"
	"github.com/lhjnilsson/foreverbull/tests/helper"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/suite"
	"go.uber.org/fx"
)

type ServiceModuleTest struct {
	suite.Suite

	app *fx.App
}

func TestModuleService(t *testing.T) {
	suite.Run(t, new(ServiceModuleTest))
}

func (test *ServiceModuleTest) SetupTest() {
	helper.SetupEnvironment(test.T(), &helper.Containers{
		Postgres: true,
		NATS:     true,
		Loki:     true,
	})
	pool, err := pgxpool.New(context.Background(), environment.GetPostgresURL())
	test.NoError(err)
	err = repository.Recreate(context.Background(), pool)
	test.NoError(err)
	test.app = fx.New(
		fx.Provide(
			func() (nats.JetStreamContext, error) {
				return stream.NewJetstream()
			},
			func() *pgxpool.Pool {
				return pool
			},
			func() *gin.Engine {
				return h.NewEngine()
			},
		),
		fx.Invoke(
			h.NewLifeCycleRouter,
		),
		stream.OrchestrationLifecycle,
		Module,
	)
	test.NoError(test.app.Start(context.TODO()))
}

func (test *ServiceModuleTest) TearDownTest() {
	helper.WaitTillContainersAreRemoved(test.T(), environment.GetDockerNetworkName(), time.Second*20)
	test.NoError(test.app.Stop(context.Background()))
}

func (test *ServiceModuleTest) TestAPIClient() {
	var client api.Client
	var err error
	// Delete image in case it exists and end with remove to cleanup
	d, err := docker.NewClientWithOpts(docker.FromEnv, docker.WithAPIVersionNegotiation())
	test.Require().NoError(err)
	_, _ = d.ImageRemove(context.TODO(), "docker.io/library/python:3.12-alpine", types.ImageRemoveOptions{})
	defer func() {
		_, _ = d.ImageRemove(context.TODO(), "docker.io/library/python:3.12-alpine", types.ImageRemoveOptions{})
	}()

	pool, err := pgxpool.New(context.Background(), environment.GetPostgresURL())
	test.Require().NoError(err)
	services := repository.Service{Conn: pool}
	instances := repository.Instance{Conn: pool}
	testService, err := services.Create(context.Background(), "docker.io/library/python:3.12-alpine")
	test.Require().NoError(err)
	testInstance, err := instances.Create(context.Background(), "instance_123", testService.Image)
	test.Require().NoError(err)

	test.Run("Create Client", func() {
		client, err = api.NewClient()
		test.NoError(err)
	})
	test.Run("TestListServices", func() {
		services, err := client.ListServices(context.Background())
		test.NoError(err)
		test.NotNil(services)
		test.Len(*services, 1)
		test.Equal(testService.Image, (*services)[0].Image)
	})
	test.Run("TestGetService", func() {
		service, err := client.GetService(context.Background(), testService.Image)
		test.NoError(err)
		test.NotNil(service)
		test.Equal(testService.Image, service.Image)
	})
	test.Run("TestGetService, Not stored", func() {
		service, err := client.GetService(context.Background(), "not_stored")
		test.Error(err)
		test.Nil(service)
	})
	test.Run("TestListInstances", func() {
		instances, err := client.ListInstances(context.Background(), "service")
		test.NoError(err)
		test.NotNil(instances)
	})
	test.Run("TestListInstances, Not stored", func() {
		instances, err := client.ListInstances(context.Background(), "not_stored")
		test.NoError(err)
		test.Empty(*instances)
	})
	test.Run("TestGetInstance", func() {
		instance, err := client.GetInstance(context.Background(), testInstance.ID)
		test.NoError(err)
		test.NotNil(instance)
		test.Equal(testInstance.ID, instance.ID)
	})
	test.Run("TestGetInstance, Not stored", func() {
		instance, err := client.GetInstance(context.Background(), "not_stored")
		test.Error(err)
		test.Nil(instance)
	})
	test.Run("TestGetImage, Not Stored", func() {
		_, err := client.GetImage(context.Background(), "docker.io/library/python:3.12-alpine")
		test.Error(err)
	})
	test.Run("TestDownloadImage", func() {
		_, err := client.DownloadImage(context.Background(), "docker.io/library/python:3.12-alpine")
		test.NoError(err)
	})
	test.Run("TestGetImage", func() {
		_, err := client.DownloadImage(context.Background(), "docker.io/library/python:3.12-alpine")
		test.NoError(err)
	})
}

func (test *ServiceModuleTest) TestCreateService() {
	images := os.Getenv("IMAGES")
	if images == "" {
		test.T().Skip("images not set")
	}

	type ServiceResponse struct {
		Image    string
		Parallel bool
		Statuses []struct {
			Status string
		}
	}
	type TestCase struct {
		Image string
	}

	testCases := []TestCase{}
	for _, image := range strings.Split(images, ",") {
		testCases = append(testCases, TestCase{Image: image})
	}
	for _, testcase := range testCases {
		test.Run(testcase.Image, func() {
			payload := `{"image": "` + testcase.Image + `"}`
			rsp := helper.Request(test.T(), http.MethodPost, "/service/api/services", payload)
			if !test.Equal(http.StatusCreated, rsp.StatusCode) {
				rspData, _ := io.ReadAll(rsp.Body)
				test.Failf("Failed to create service: %s", string(rspData))
			}
			condition := func() (bool, error) {
				rsp = helper.Request(test.T(), http.MethodGet, "/service/api/services/"+testcase.Image, nil)
				if rsp.StatusCode != http.StatusOK {
					return false, fmt.Errorf("failed to get service: %d", rsp.StatusCode)
				}
				data := &ServiceResponse{}
				err := json.NewDecoder(rsp.Body).Decode(data)
				if err != nil {
					return false, fmt.Errorf("failed to decode response: %s", err.Error())
				}
				if data.Statuses[0].Status != "READY" {
					return false, nil
				}
				return true, nil
			}
			err := helper.WaitUntilCondition(test.T(), condition, time.Second*10)
			test.NoError(err)
			rsp = helper.Request(test.T(), http.MethodGet, "/service/api/services/"+testcase.Image, nil)
			if !test.Equal(http.StatusOK, rsp.StatusCode) {
				rspData, _ := io.ReadAll(rsp.Body)
				test.Failf("Failed to get service: %s", string(rspData))
			}
			data := &ServiceResponse{}
			err = json.NewDecoder(rsp.Body).Decode(data)
			if err != nil {
				test.Failf("Failed to decode response: %w", err.Error())
				return
			}
		})
	}
}
