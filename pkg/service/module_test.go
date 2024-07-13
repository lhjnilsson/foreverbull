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

	"github.com/docker/docker/api/types/image"
	docker "github.com/docker/docker/client"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	h "github.com/lhjnilsson/foreverbull/internal/http"
	"github.com/lhjnilsson/foreverbull/internal/stream"
	"github.com/lhjnilsson/foreverbull/internal/test_helper"
	"github.com/lhjnilsson/foreverbull/pkg/finance"
	"github.com/lhjnilsson/foreverbull/pkg/service/api"
	"github.com/lhjnilsson/foreverbull/pkg/service/internal/container"
	"github.com/lhjnilsson/foreverbull/pkg/service/internal/repository"
	"github.com/lhjnilsson/foreverbull/pkg/service/worker"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/suite"
	"go.uber.org/fx"
)

type ServiceModuleTest struct {
	suite.Suite

	app *fx.App
}

func TestModuleService(t *testing.T) {
	_, found := os.LookupEnv("IMAGES")
	if !found {
		t.Skip("images not set")
	}
	suite.Run(t, new(ServiceModuleTest))
}

func (test *ServiceModuleTest) SetupTest() {
	test_helper.SetupEnvironment(test.T(), &test_helper.Containers{
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
			func() (*nats.Conn, nats.JetStreamContext, error) {
				return stream.New()
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
		finance.Module,
		Module,
	)
	test.Require().NoError(test.app.Start(context.TODO()))
}

func (test *ServiceModuleTest) TearDownTest() {
	test_helper.WaitTillContainersAreRemoved(test.T(), environment.GetDockerNetworkName(), time.Second*20)
	test.NoError(test.app.Stop(context.Background()))
}

// TODO: Fix this test, fails to create network
func (test *ServiceModuleTest) NoTestAPIClient() {
	var client api.Client
	var err error
	// Delete image in case it exists and end with remove to cleanup
	d, err := docker.NewClientWithOpts(docker.FromEnv, docker.WithAPIVersionNegotiation())
	test.Require().NoError(err)
	_, _ = d.ImageRemove(context.TODO(), "docker.io/library/python:3.12-alpine", image.RemoveOptions{})
	defer func() {
		_, _ = d.ImageRemove(context.TODO(), "docker.io/library/python:3.12-alpine", image.RemoveOptions{})
	}()

	pool, err := pgxpool.New(context.Background(), environment.GetPostgresURL())
	test.Require().NoError(err)
	services := repository.Service{Conn: pool}
	instances := repository.Instance{Conn: pool}
	testService, err := services.Create(context.Background(), "docker.io/library/python:3.12-alpine")
	test.Require().NoError(err)
	testInstance, err := instances.Create(context.Background(), "instance_123", &testService.Image)
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
	client, err := api.NewClient()
	test.Require().NoError(err)

	type ServiceResponse struct {
		Image    string
		Statuses []struct {
			Status string
		}
	}
	type TestCase struct {
		Image string
	}

	testCases := []TestCase{}
	for _, image := range strings.Split(os.Getenv("IMAGES"), ",") {
		testCases = append(testCases, TestCase{Image: image})
	}
	for _, testcase := range testCases {
		test.Run("Create_"+testcase.Image, func() {
			payload := `{"image": "` + testcase.Image + `"}`
			rsp := test_helper.Request(test.T(), http.MethodPost, "/service/api/services", payload)
			if !test.Equal(http.StatusCreated, rsp.StatusCode) {
				rspData, _ := io.ReadAll(rsp.Body)
				test.Failf("Failed to create service: %s", string(rspData))
			}
			condition := func() (bool, error) {
				rsp = test_helper.Request(test.T(), http.MethodGet, "/service/api/services/"+testcase.Image, nil)
				if rsp.StatusCode != http.StatusOK {
					return false, fmt.Errorf("failed to get service: %d", rsp.StatusCode)
				}
				data := &ServiceResponse{}
				err := json.NewDecoder(rsp.Body).Decode(data)
				if err != nil {
					return false, fmt.Errorf("failed to decode response: %s", err.Error())
				}
				switch data.Statuses[0].Status {
				case "READY":
					return true, nil
				case "ERROR":
					return false, fmt.Errorf("service in error state")
				case "STOPPED":
					return false, fmt.Errorf("service in stopped state")
				}
				fmt.Println("STATUS: ", data.Statuses[0].Status)
				return false, nil
			}
			err := test_helper.WaitUntilCondition(test.T(), condition, time.Second*10)
			test.NoError(err)
			rsp = test_helper.Request(test.T(), http.MethodGet, "/service/api/services/"+testcase.Image, nil)
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
		test.Run("Configure_"+testcase.Image, func() {
			pool, err := pgxpool.New(context.Background(), environment.GetPostgresURL())
			test.NoError(err)

			instanceID := uuid.New().String()

			instances := repository.Instance{Conn: pool}
			_, err = instances.Create(context.Background(), instanceID, &testcase.Image)
			test.NoError(err)

			c, err := container.NewContainerRegistry()
			test.Require().NoError(err)

			_, err = c.Start(context.Background(), testcase.Image, instanceID, nil)
			test.Require().NoError(err)

			condition := func() (bool, error) {
				instance, err := client.GetInstance(context.Background(), instanceID)
				if err != nil {
					return false, fmt.Errorf("failed to get instance: %w", err)
				}
				switch instance.Statuses[0].Status {
				case "RUNNING":
					return true, nil
				case "ERROR":
					return false, fmt.Errorf("instance in error state")
				case "STOPPED":
					return false, fmt.Errorf("instance in stopped state")
				}
				return false, nil
			}
			err = test_helper.WaitUntilCondition(test.T(), condition, time.Second*10)
			test.NoError(err)

			service, err := client.GetService(context.Background(), testcase.Image)
			test.NoError(err)
			functions, err := service.Algorithm.Configure()
			test.NoError(err)
			wPool, err := worker.NewPool(context.Background(), service.Algorithm)
			test.NoError(err)

			configRequest := api.ConfigureInstanceRequest{
				BrokerPort:    wPool.GetPort(),
				NamespacePort: wPool.GetNamespacePort(),
				DatabaseURL:   environment.GetPostgresURL(),
				Functions:     functions,
			}

			instance, err := client.ConfigureInstance(context.Background(), instanceID, &configRequest)
			test.NoError(err)
			test.NotNil(instance)

			test.NoError(client.StopInstance(context.Background(), instanceID))

			test.NoError(wPool.Close())
		})
	}
}
