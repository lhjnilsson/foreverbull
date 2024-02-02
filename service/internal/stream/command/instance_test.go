package command

import (
	"context"
	"errors"
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

func TestIC(t *testing.T) {
	suite.Run(t, new(InstanceTest))
}

func (s *InstanceTest) SetupTest() {
	helper.SetupEnvironment(s.T(), &helper.Containers{
		Postgres: true,
	})

	var err error
	s.db, err = pgxpool.New(context.Background(), environment.GetPostgresURL())
	s.NoError(err)

	err = repository.Recreate(context.TODO(), s.db)
	s.NoError(err)

	services := repository.Service{Conn: s.db}
	instances := repository.Instance{Conn: s.db}
	s.testService, err = services.Create(context.TODO(), "test-service", "test-image")
	s.NoError(err)

	err = services.UpdateServiceInfo(context.Background(), s.testService.Name, "test-service-type", nil)
	s.NoError(err)

	instanceID := uuid.New().String()
	s.testInstance, err = instances.Create(context.TODO(), instanceID, s.testService.Name)
	s.NoError(err)

	s.serviceInstance = helper.NewServiceInstance(s.T())
	err = instances.UpdateHostPort(context.Background(), s.testInstance.ID, s.serviceInstance.Host, s.serviceInstance.Port)
	s.NoError(err)

	err = instances.UpdateStatus(context.Background(), s.testInstance.ID, entity.InstanceStatusRunning, nil)
	s.NoError(err)
}

func (s *InstanceTest) TearDownTest() {
	s.NoError(s.serviceInstance.Close())
}

func (s *InstanceTest) SetupSubTest() {
	s.serviceInstance = helper.NewServiceInstance(s.T())

	instances := repository.Instance{Conn: s.db}
	err := instances.UpdateHostPort(context.Background(), s.testInstance.ID, s.serviceInstance.Host, s.serviceInstance.Port)
	s.NoError(err)

	err = instances.UpdateStatus(context.Background(), s.testInstance.ID, entity.InstanceStatusRunning, nil)
	s.NoError(err)
}

func (s *InstanceTest) TearDownSubTest() {
	s.NoError(s.serviceInstance.Close())
}

func (s *InstanceTest) TestInstanceInterviewFailGetInfo() {
	b := new(mockStream.Message)
	b.On("MustGet", stream.DBDep).Return(s.db)
	b.On("ParsePayload", &st.InstanceInterviewCommand{}).Return(nil).Run(func(args mock.Arguments) {
		payload := args.Get(0).(*st.InstanceInterviewCommand)
		payload.ID = s.testInstance.ID
	})
	ctx := context.Background()
	commandCtx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	responses := map[string][]byte{
		"info": []byte(`{"task": "info", "data":`),
	}
	go s.serviceInstance.Process(commandCtx, responses)

	err := InstanceInterview(commandCtx, b)
	s.Error(err)
	s.EqualError(err, "error reading instance info: error decoding message: unexpected end of JSON input")

	err = s.serviceInstance.Close()
	s.NoError(err)
}

func (s *InstanceTest) TestInstanceInterviewSuccessful() {
	services := repository.Service{Conn: s.db}

	b := new(mockStream.Message)
	b.On("MustGet", stream.DBDep).Return(s.db)
	b.On("ParsePayload", &st.InstanceInterviewCommand{}).Return(nil).Run(func(args mock.Arguments) {
		payload := args.Get(0).(*st.InstanceInterviewCommand)
		payload.ID = s.testInstance.ID
	})

	type TestCase struct {
		Payload      string
		ExpectedType string
		Parameters   []entity.Parameter
	}

	testCases := []TestCase{
		{
			Payload:      `{"type":"backtest","parameters":[]}`,
			ExpectedType: "backtest",
			Parameters:   []entity.Parameter{},
		},
		{
			Payload:      `{"type":"worker","parameters":[{"key": "param1", "type": "int", "default": "3"}]}`,
			ExpectedType: "worker",
			Parameters: []entity.Parameter{
				{
					Key:     "param1",
					Type:    "int",
					Value:   "",
					Default: "3",
				},
			},
		},
	}
	for _, testCase := range testCases {
		s.Run(testCase.ExpectedType, func() {
			ctx := context.Background()
			commandCtx, cancel := context.WithTimeout(ctx, time.Second)
			defer cancel()
			responses := map[string][]byte{
				"info": []byte(`{"task": "info", "data":` + testCase.Payload + `}`),
			}
			go s.serviceInstance.Process(commandCtx, responses)

			err := InstanceInterview(commandCtx, b)
			s.NoError(err)

			service, err := services.Get(context.Background(), s.testService.Name)
			s.NoError(err)
			s.Equal(testCase.ExpectedType, *service.Type)
			s.Equal(testCase.Parameters, *service.WorkerParameters)
		})
	}
}

func (s *InstanceTest) TestInstanceSanityCheckMissmatchServiceTypel() {
	b := new(mockStream.Message)
	b.On("MustGet", stream.DBDep).Return(s.db)
	b.On("ParsePayload", &st.InstanceSanityCheckCommand{}).Return(nil).Run(func(args mock.Arguments) {
		payload := args.Get(0).(*st.InstanceSanityCheckCommand)
		payload.IDs = []string{s.testInstance.ID}
	})
	ctx := context.Background()
	commandCtx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()

	responses := map[string][]byte{
		"info": []byte(`{"task": "info", "data": {"type": "bad-type", "parameters": []}}`),
	}
	go s.serviceInstance.Process(commandCtx, responses)

	err := InstanceSanityCheck(commandCtx, b)
	s.Error(err)
	s.ErrorContains(err, "test-service-type != bad-type")
}

func (s *InstanceTest) TestInstanceSanityCheckSuccessful() {
	b := new(mockStream.Message)
	b.On("MustGet", stream.DBDep).Return(s.db)
	b.On("ParsePayload", &st.InstanceSanityCheckCommand{}).Return(nil).Run(func(args mock.Arguments) {
		payload := args.Get(0).(*st.InstanceSanityCheckCommand)
		payload.IDs = []string{s.testInstance.ID}
	})
	ctx := context.Background()
	commandCtx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()

	responses := map[string][]byte{
		"info": []byte(`{"task": "info", "data": {"type": "test-service-type", "parameters": []}}`),
	}
	go s.serviceInstance.Process(commandCtx, responses)

	err := InstanceSanityCheck(commandCtx, b)
	s.NoError(err)
}

func (s *InstanceTest) TestInstanceStopFail() {
	b := new(mockStream.Message)
	c := new(mockContainer.Container)
	c.On("Stop", mock.Anything, s.testInstance.ID, true).Return(errors.New("error stopping instance"))

	b.On("MustGet", serviceDependency.ContainerDep).Return(c)

	b.On("ParsePayload", &st.InstanceStopCommand{}).Return(nil).Run(func(args mock.Arguments) {
		payload := args.Get(0).(*st.InstanceStopCommand)
		payload.ID = s.testInstance.ID
	})
	ctx := context.Background()
	commandCtx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()

	err := InstanceStop(commandCtx, b)
	s.Error(err)
	s.EqualError(err, "error stopping instance")
}

func (s *InstanceTest) TestInstanceStopSuccessful() {
	b := new(mockStream.Message)
	c := new(mockContainer.Container)
	c.On("Stop", mock.Anything, s.testInstance.ID, true).Return(nil)

	b.On("MustGet", serviceDependency.ContainerDep).Return(c)

	b.On("ParsePayload", &st.InstanceStopCommand{}).Return(nil).Run(func(args mock.Arguments) {
		payload := args.Get(0).(*st.InstanceStopCommand)
		payload.ID = s.testInstance.ID
	})
	ctx := context.Background()
	commandCtx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()

	err := InstanceStop(commandCtx, b)
	s.NoError(err)
}
