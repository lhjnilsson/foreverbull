package command

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/internal/stream"
	"github.com/lhjnilsson/foreverbull/service/entity"
	"github.com/lhjnilsson/foreverbull/service/internal/repository"
	serviceDependency "github.com/lhjnilsson/foreverbull/service/internal/stream/dependency"
	st "github.com/lhjnilsson/foreverbull/service/stream"
	"github.com/lhjnilsson/foreverbull/tests/helper"
	mockStream "github.com/lhjnilsson/foreverbull/tests/mocks/internal_/stream"
	mockContainer "github.com/lhjnilsson/foreverbull/tests/mocks/service/container"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type InstanceTest struct {
	suite.Suite

	db *pgxpool.Pool

	testService  *entity.Service
	testInstance *entity.Instance

	serviceInstance *helper.ServiceInstance
}

func TestInstanceCommand(t *testing.T) {
	suite.Run(t, new(InstanceTest))
}

func (test *InstanceTest) SetupTest() {
	helper.SetupEnvironment(test.T(), &helper.Containers{
		Postgres: true,
	})

	var err error
	test.db, err = pgxpool.New(context.Background(), environment.GetPostgresURL())
	test.Require().NoError(err)

	err = repository.Recreate(context.TODO(), test.db)
	test.Require().NoError(err)

	services := repository.Service{Conn: test.db}
	instances := repository.Instance{Conn: test.db}
	test.testService, err = services.Create(context.TODO(), "test-image")
	test.Require().NoError(err)

	instanceID := uuid.New().String()
	test.testInstance, err = instances.Create(context.TODO(), instanceID, test.testService.Image)
	test.Require().NoError(err)

	test.serviceInstance = helper.NewServiceInstance(test.T())
	err = instances.UpdateHostPort(context.Background(), test.testInstance.ID, test.serviceInstance.Host, test.serviceInstance.Port)
	test.Require().NoError(err)

	err = instances.UpdateStatus(context.Background(), test.testInstance.ID, entity.InstanceStatusRunning, nil)
	test.Require().NoError(err)
}

func (test *InstanceTest) TearDownTest() {
	test.NoError(test.serviceInstance.Close())
}

func (test *InstanceTest) SetupSubTest() {
	test.serviceInstance = helper.NewServiceInstance(test.T())

	instances := repository.Instance{Conn: test.db}
	err := instances.UpdateHostPort(context.Background(), test.testInstance.ID, test.serviceInstance.Host, test.serviceInstance.Port)
	test.NoError(err)

	err = instances.UpdateStatus(context.Background(), test.testInstance.ID, entity.InstanceStatusRunning, nil)
	test.NoError(err)
}

func (test *InstanceTest) TearDownSubTest() {
	test.NoError(test.serviceInstance.Close())
}

func (test *InstanceTest) TestInstanceInterviewFailGetInfo() {
	b := new(mockStream.Message)
	b.On("MustGet", stream.DBDep).Return(test.db)
	b.On("ParsePayload", &st.InstanceInterviewCommand{}).Return(nil).Run(func(args mock.Arguments) {
		payload := args.Get(0).(*st.InstanceInterviewCommand)
		payload.ID = test.testInstance.ID
	})
	ctx := context.Background()
	commandCtx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	responses := map[string][]byte{
		"info": []byte(`{"task": "info", "data":`),
	}
	go test.serviceInstance.Process(commandCtx, responses)

	err := InstanceInterview(commandCtx, b)
	test.Error(err)
	test.EqualError(err, "error reading instance info: error decoding message: unexpected end of JSON input")

	err = test.serviceInstance.Close()
	test.NoError(err)
}

func (test *InstanceTest) TestInstanceInterviewSuccessful() {
	services := repository.Service{Conn: test.db}

	b := new(mockStream.Message)
	b.On("MustGet", stream.DBDep).Return(test.db)
	b.On("ParsePayload", &st.InstanceInterviewCommand{}).Return(nil).Run(func(args mock.Arguments) {
		payload := args.Get(0).(*st.InstanceInterviewCommand)
		payload.ID = test.testInstance.ID
	})

	type TestCase struct {
		assetFileName     string
		expectedAlgorithm entity.ServiceAlgorithm
	}

	testCases := []TestCase{
		{
			assetFileName: "service1.json",
			expectedAlgorithm: entity.ServiceAlgorithm{
				FilePath: "algo1.py",
				Functions: []entity.ServiceFunction{
					{
						Name: "handle_data",
						Parameters: []entity.ServiceFunctionParameter{
							{
								Key:  "low",
								Type: "int",
							},
							{
								Key:  "high",
								Type: "int",
							},
						},
						ParallelExecution: false,
						ReturnType:        entity.ListOfOrdersReturnType,
						InputKey:          "symbols",
					},
				},
				Namespace: map[string]entity.Namespace{},
			},
		},
		{
			assetFileName: "service2.json",
			expectedAlgorithm: entity.ServiceAlgorithm{
				FilePath: "algo2.py",
				Functions: []entity.ServiceFunction{
					{
						Name: "handle_data",
						Parameters: []entity.ServiceFunctionParameter{
							{
								Key: "low",
								Default: func() *string {
									v := "5"
									return &v
								}(),
								Type: "int",
							},
							{
								Key: "high",
								Default: func() *string {
									v := "10"
									return &v
								}(),
								Type: "int",
							},
						},
						ParallelExecution: true,
						ReturnType:        entity.OrderReturnType,
						InputKey:          "symbols",
					},
				},
				Namespace: map[string]entity.Namespace{},
			},
		},
		{
			assetFileName: "service3.json",
			expectedAlgorithm: entity.ServiceAlgorithm{
				FilePath: "algo3.py",
				Functions: []entity.ServiceFunction{
					{
						Name: "handle_data",
						Parameters: []entity.ServiceFunctionParameter{
							{
								Key: "low",
								Default: func() *string {
									v := "5"
									return &v
								}(),
								Type: "int",
							},
							{
								Key: "high",
								Default: func() *string {
									v := "10"
									return &v
								}(),
								Type: "int",
							},
						},
						ParallelExecution: true,
						ReturnType:        entity.OrderReturnType,
						InputKey:          "symbols",
					},
				},
				Namespace: map[string]entity.Namespace{
					"qualified_symbols": {
						Type:      "array",
						ValueType: "string",
					},
					"rsi": {
						Type:      "object",
						ValueType: "float",
					},
				},
			},
		},
		{
			assetFileName: "service4.json",
			expectedAlgorithm: entity.ServiceAlgorithm{
				FilePath: "algo4.py",
				Functions: []entity.ServiceFunction{
					{
						Name:              "filter_assets",
						Parameters:        []entity.ServiceFunctionParameter{},
						ParallelExecution: false,
						ReturnType:        entity.NamespaceValueReturnType,
						InputKey:          "symbols",
						NamespaceReturnKey: func() *string {
							v := "qualified_symbols"
							return &v
						}(),
					},
					{
						Name: "measure_assets",
						Parameters: []entity.ServiceFunctionParameter{
							{
								Key: "low",
								Default: func() *string {
									v := "5"
									return &v
								}(),
								Type: "int",
							},
							{
								Key: "high",
								Default: func() *string {
									v := "10"
									return &v
								}(),
								Type: "int",
							},
						},
						ParallelExecution: true,
						ReturnType:        entity.NamespaceValueReturnType,
						InputKey:          "qualified_symbols",
						NamespaceReturnKey: func() *string {
							v := "asset_metrics"
							return &v
						}(),
					},
					{
						Name:               "create_orders",
						Parameters:         []entity.ServiceFunctionParameter{},
						ParallelExecution:  false,
						ReturnType:         entity.ListOfOrdersReturnType,
						InputKey:           "asset_metrics",
						NamespaceReturnKey: nil,
					},
				},
				Namespace: map[string]entity.Namespace{
					"qualified_symbols": {
						Type:      "array",
						ValueType: "string",
					},
					"asset_metrics": {
						Type:      "object",
						ValueType: "float",
					},
				},
			},
		},
	}
	for _, testCase := range testCases {
		test.Run(testCase.assetFileName, func() {
			payload, err := helper.GetAsset(test.T(), testCase.assetFileName)
			test.Require().NoError(err)

			ctx := context.Background()
			commandCtx, cancel := context.WithTimeout(ctx, time.Second)
			defer cancel()
			responses := map[string][]byte{
				"info": []byte(`{"task": "info", "data":` + string(payload) + `}`),
			}
			go test.serviceInstance.Process(commandCtx, responses)

			err = InstanceInterview(commandCtx, b)
			test.NoError(err)

			service, err := services.Get(context.Background(), test.testService.Image)
			test.NoError(err)
			fmt.Println("SERVICE: ", service.Algorithm)
			test.Require().NotNil(service.Algorithm)
			test.Equal(testCase.expectedAlgorithm, *service.Algorithm)
		})
	}
}

func (test *InstanceTest) TestInstanceSanityCheckSuccessful() {
	b := new(mockStream.Message)
	b.On("MustGet", stream.DBDep).Return(test.db)
	b.On("ParsePayload", &st.InstanceSanityCheckCommand{}).Return(nil).Run(func(args mock.Arguments) {
		payload := args.Get(0).(*st.InstanceSanityCheckCommand)
		payload.IDs = []string{test.testInstance.ID}
	})
	ctx := context.Background()
	commandCtx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()

	responses := map[string][]byte{
		"info": []byte(`{"task": "info", "data": {"type": "test-service-type", "parameters": []}}`),
	}
	go test.serviceInstance.Process(commandCtx, responses)

	err := InstanceSanityCheck(commandCtx, b)
	test.NoError(err)
}

func (test *InstanceTest) TestInstanceStopFail() {
	b := new(mockStream.Message)
	c := new(mockContainer.Container)
	c.On("Stop", mock.Anything, test.testInstance.ID, true).Return(errors.New("error stopping instance"))

	b.On("MustGet", serviceDependency.ContainerDep).Return(c)

	b.On("ParsePayload", &st.InstanceStopCommand{}).Return(nil).Run(func(args mock.Arguments) {
		payload := args.Get(0).(*st.InstanceStopCommand)
		payload.ID = test.testInstance.ID
	})
	ctx := context.Background()
	commandCtx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()

	err := InstanceStop(commandCtx, b)
	test.Error(err)
	test.EqualError(err, "error stopping instance")
}

func (test *InstanceTest) TestInstanceStopSuccessful() {
	b := new(mockStream.Message)
	c := new(mockContainer.Container)
	c.On("Stop", mock.Anything, test.testInstance.ID, true).Return(nil)

	b.On("MustGet", serviceDependency.ContainerDep).Return(c)

	b.On("ParsePayload", &st.InstanceStopCommand{}).Return(nil).Run(func(args mock.Arguments) {
		payload := args.Get(0).(*st.InstanceStopCommand)
		payload.ID = test.testInstance.ID
	})
	ctx := context.Background()
	commandCtx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()

	err := InstanceStop(commandCtx, b)
	test.NoError(err)
}
