package command

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/config"
	"github.com/lhjnilsson/foreverbull/internal/stream"
	"github.com/lhjnilsson/foreverbull/service/entity"
	"github.com/lhjnilsson/foreverbull/service/internal/repository"
	serviceDependency "github.com/lhjnilsson/foreverbull/service/internal/stream/dependency"
	ss "github.com/lhjnilsson/foreverbull/service/stream"
	"github.com/lhjnilsson/foreverbull/tests/helper"
	mockStream "github.com/lhjnilsson/foreverbull/tests/mocks/internal_/stream"
	mockContainer "github.com/lhjnilsson/foreverbull/tests/mocks/service/container"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type ServiceTest struct {
	suite.Suite

	config *config.Config
	db     *pgxpool.Pool

	testService *entity.Service
}

func TestServiceCommands(t *testing.T) {
	suite.Run(t, new(ServiceTest))
}

func (s *ServiceTest) SetupTest() {
	var err error
	config := helper.TestingConfig(s.T(), &helper.Containers{
		Postgres: true,
	})

	s.db, err = pgxpool.New(context.Background(), config.PostgresURI)
	s.NoError(err)

	err = repository.Recreate(context.TODO(), s.db)
	s.NoError(err)

	services := repository.Service{Conn: s.db}
	s.testService, err = services.Create(context.TODO(), "test-service", "test-image")
	s.NoError(err)
}

func (s *ServiceTest) TearDownSuite() {
}

func (test *ServiceTest) TestUpdateServiceStatus() {
	m := new(mockStream.Message)
	m.On("MustGet", stream.DBDep).Return(test.db)

	services := repository.Service{Conn: test.db}

	type TestCase struct {
		Status entity.ServiceStatusType
		Error  error
	}
	testCases := []TestCase{
		{entity.ServiceStatusInterview, nil},
		{entity.ServiceStatusReady, nil},
		{entity.ServiceStatusError, errors.New("test-error")},
	}
	for _, tc := range testCases {
		test.Run(string(tc.Status), func() {
			m.On("ParsePayload", &ss.UpdateServiceStatusCommand{}).Return(nil).Run(func(args mock.Arguments) {
				command := args.Get(0).(*ss.UpdateServiceStatusCommand)
				command.Name = test.testService.Name
				command.Status = tc.Status
				command.Error = tc.Error
			})

			ctx := context.Background()
			err := UpdateServiceStatus(ctx, m)
			test.NoError(err)

			service, err := services.Get(ctx, test.testService.Name)
			test.NoError(err)
			test.Equal(tc.Status, service.Statuses[0].Status)
			if tc.Error != nil {
				test.Equal(tc.Error.Error(), *service.Statuses[0].Error)
			} else {
				test.Nil(service.Statuses[0].Error)
			}
		})
	}
}

func (s *ServiceTest) TestServiceStartFail() {
	b := new(mockStream.Message)
	b.On("ParsePayload", &ss.ServiceStartCommand{}).Return(nil).Run(func(args mock.Arguments) {
		command := args.Get(0).(*ss.ServiceStartCommand)
		command.Name = s.testService.Name
		command.InstanceID = "test-instance"
	})

	c := new(mockContainer.Container)
	b.On("MustGet", stream.DBDep).Return(s.db)
	b.On("MustGet", serviceDependency.ContainerDep).Return(c)
	b.On("MustGet", stream.ConfigDep).Return(s.config)

	c.On("Start", mock.Anything, s.config, s.testService.Name, s.testService.Image, "test-instance").Return("", errors.New("fail to start"))

	err := ServiceStart(context.Background(), b)
	s.Error(err)
	s.EqualError(err, "error starting container: fail to start")
}

func (s *ServiceTest) TestServiceStartSuccessful() {
	b := new(mockStream.Message)
	b.On("ParsePayload", &ss.ServiceStartCommand{}).Return(nil).Run(func(args mock.Arguments) {
		command := args.Get(0).(*ss.ServiceStartCommand)
		command.Name = s.testService.Name
		command.InstanceID = "test-instance"
	})

	c := new(mockContainer.Container)
	b.On("MustGet", stream.DBDep).Return(s.db)
	b.On("MustGet", serviceDependency.ContainerDep).Return(c)
	b.On("MustGet", stream.ConfigDep).Return(s.config)

	c.On("Start", mock.Anything, s.config, s.testService.Name, s.testService.Image, "test-instance").Return("test-container-id", nil)

	err := ServiceStart(context.Background(), b)
	s.NoError(err)

	instances := repository.Instance{Conn: s.db}
	instance, err := instances.Get(context.Background(), "test-instance")
	s.NoError(err)
	s.Equal("test-instance", instance.ID)
	s.Equal("test-service", instance.Service)
}
