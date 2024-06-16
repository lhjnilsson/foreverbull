package command

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/internal/stream"
	"github.com/lhjnilsson/foreverbull/internal/test_helper"
	"github.com/lhjnilsson/foreverbull/pkg/service/container"
	"github.com/lhjnilsson/foreverbull/pkg/service/entity"
	"github.com/lhjnilsson/foreverbull/pkg/service/internal/repository"
	serviceDependency "github.com/lhjnilsson/foreverbull/pkg/service/internal/stream/dependency"
	ss "github.com/lhjnilsson/foreverbull/pkg/service/stream"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type ServiceTest struct {
	suite.Suite

	db *pgxpool.Pool

	testService *entity.Service
}

func TestServiceCommands(t *testing.T) {
	suite.Run(t, new(ServiceTest))
}

func (test *ServiceTest) SetupSuite() {
	test_helper.SetupEnvironment(test.T(), &test_helper.Containers{
		Postgres: true,
	})

}

func (test *ServiceTest) SetupTest() {
	var err error
	test.db, err = pgxpool.New(context.Background(), environment.GetPostgresURL())
	test.Require().NoError(err)

	err = repository.Recreate(context.TODO(), test.db)
	test.Require().NoError(err)

	services := repository.Service{Conn: test.db}
	test.testService, err = services.Create(context.TODO(), "test-image")
	test.Require().NoError(err)
}

func (test *ServiceTest) TearDownSuite() {
}

func (test *ServiceTest) TestUpdateServiceStatus() {
	m := new(stream.MockMessage)
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
				command.Image = test.testService.Image
				command.Status = tc.Status
				command.Error = tc.Error
			})

			ctx := context.Background()
			err := UpdateServiceStatus(ctx, m)
			test.NoError(err)

			service, err := services.Get(ctx, test.testService.Image)
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

func (test *ServiceTest) TestServiceStart() {
	test.Run("fail to start", func() {
		b := new(stream.MockMessage)
		b.On("ParsePayload", &ss.ServiceStartCommand{}).Return(nil).Run(func(args mock.Arguments) {
			command := args.Get(0).(*ss.ServiceStartCommand)
			command.Image = test.testService.Image
			command.InstanceID = "test-instance"
		})
		b.On("GetOrchestrationID").Return("test-orchestration-id")

		c := new(container.MockContainer)
		b.On("MustGet", stream.DBDep).Return(test.db)
		b.On("MustGet", serviceDependency.ContainerDep).Return(c)

		c.On("Start", mock.Anything, test.testService.Image, "test-instance",
			map[string]string{"orchestration_id": "test-orchestration-id"}).Return("", errors.New("fail to start"))

		err := ServiceStart(context.Background(), b)
		test.Error(err)
		test.EqualError(err, "error starting container: fail to start")
	})
	test.Run("successful", func() {
		b := new(stream.MockMessage)
		b.On("ParsePayload", &ss.ServiceStartCommand{}).Return(nil).Run(func(args mock.Arguments) {
			command := args.Get(0).(*ss.ServiceStartCommand)
			command.Image = test.testService.Image
			command.InstanceID = "test-instance"
		})
		b.On("GetOrchestrationID").Return("test-orchestration-id")

		c := new(container.MockContainer)
		b.On("MustGet", stream.DBDep).Return(test.db)
		b.On("MustGet", serviceDependency.ContainerDep).Return(c)

		c.On("Start", mock.Anything, test.testService.Image, "test-instance",
			map[string]string{"orchestration_id": "test-orchestration-id"}).Return("test-container-id", nil)

		err := ServiceStart(context.Background(), b)
		test.NoError(err)

		instances := repository.Instance{Conn: test.db}
		instance, err := instances.Get(context.Background(), "test-instance")
		test.NoError(err)
		test.Equal("test-instance", instance.ID)
		test.Equal("test-image", *instance.Image)
	})
}
