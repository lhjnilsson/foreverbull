package command

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	service_pb "github.com/lhjnilsson/foreverbull/internal/pb/service"
	"github.com/lhjnilsson/foreverbull/internal/stream"
	"github.com/lhjnilsson/foreverbull/internal/test_helper"
	"github.com/lhjnilsson/foreverbull/pkg/service/container"
	"github.com/lhjnilsson/foreverbull/pkg/service/entity"
	"github.com/lhjnilsson/foreverbull/pkg/service/internal/repository"
	serviceDependency "github.com/lhjnilsson/foreverbull/pkg/service/internal/stream/dependency"
	st "github.com/lhjnilsson/foreverbull/pkg/service/stream"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"google.golang.org/protobuf/proto"
)

type InstanceTest struct {
	suite.Suite

	db *pgxpool.Pool

	testService  *entity.Service
	testInstance *entity.Instance

	serviceInstance *test_helper.ServiceInstance
}

func TestInstanceCommand(t *testing.T) {
	suite.Run(t, new(InstanceTest))
}

func (test *InstanceTest) SetupSuite() {
	test_helper.SetupEnvironment(test.T(), &test_helper.Containers{
		Postgres: true,
	})
}

func (test *InstanceTest) SetupTest() {
	var err error
	test.db, err = pgxpool.New(context.Background(), environment.GetPostgresURL())
	test.Require().NoError(err)

	err = repository.Recreate(context.TODO(), test.db)
	test.Require().NoError(err)

	services := repository.Service{Conn: test.db}
	test.testService, err = services.Create(context.TODO(), "test-image")
	test.Require().NoError(err)

	instances := repository.Instance{Conn: test.db}
	test.testInstance, err = instances.Create(context.TODO(), "instanceID", &test.testService.Image)
	test.Require().NoError(err)
}

func (test *InstanceTest) TearDownTest() {
	test.NoError(test.serviceInstance.Close())
}

func (test *InstanceTest) SetupSubTest() {
	test.serviceInstance = test_helper.NewServiceInstance(test.T())

	instances := repository.Instance{Conn: test.db}
	err := instances.UpdateHostPort(context.Background(), test.testInstance.ID, test.serviceInstance.Host, test.serviceInstance.Port)
	test.NoError(err)

	err = instances.UpdateStatus(context.Background(), test.testInstance.ID, entity.InstanceStatusRunning, nil)
	test.NoError(err)
}

func (test *InstanceTest) TearDownSubTest() {
	test.NoError(test.serviceInstance.Close())
}

func (test *InstanceTest) TestInstanceInterview() {
	services := repository.Service{Conn: test.db}

	b := new(stream.MockMessage)
	b.On("MustGet", stream.DBDep).Return(test.db)
	b.On("ParsePayload", &st.InstanceInterviewCommand{}).Return(nil).Run(func(args mock.Arguments) {
		payload := args.Get(0).(*st.InstanceInterviewCommand)
		payload.ID = test.testInstance.ID
	})

	algo := service_pb.Algorithm{
		FilePath: "/file.py",
	}
	bytes, err := proto.Marshal(&algo)
	test.Require().NoError(err)

	type TestCase struct {
		Payload   []byte
		Algorithm entity.Algorithm
		err       string
	}
	testCases := []TestCase{
		{
			Payload:   bytes,
			Algorithm: entity.Algorithm{FilePath: "/file.py"},
		},
	}
	for id, testCase := range testCases {
		test.Run(fmt.Sprintf("Test%d", id), func() {
			ctx := context.Background()
			commandCtx, cancel := context.WithTimeout(ctx, time.Second)
			defer cancel()
			responses := map[string][]byte{
				"info": testCase.Payload,
			}
			go test.serviceInstance.Process(commandCtx, responses)

			err := InstanceInterview(commandCtx, b)
			if len(testCase.err) == 0 {
				test.Require().NoError(err)
				service, err := services.Get(context.Background(), test.testService.Image)
				test.NoError(err)
				test.Require().NotNil(service.Algorithm)
				test.Equal(testCase.Algorithm, *service.Algorithm)
			} else {
				test.ErrorContains(err, testCase.err)
			}
		})
	}
}

func (test *InstanceTest) TestInstanceSanityCheckSuccessful() {
	b := new(stream.MockMessage)
	b.On("MustGet", stream.DBDep).Return(test.db)
	b.On("ParsePayload", &st.InstanceSanityCheckCommand{}).Return(nil).Run(func(args mock.Arguments) {
		payload := args.Get(0).(*st.InstanceSanityCheckCommand)
		payload.IDs = []string{test.testInstance.ID}
	})

	algo := service_pb.Algorithm{
		FilePath: "/file.py",
	}
	bytes, err := proto.Marshal(&algo)
	test.Require().NoError(err)

	type TestCase struct {
		Payload []byte
		err     string
	}
	testCases := []TestCase{
		{
			Payload: bytes,
		},
	}

	for id, testCase := range testCases {
		test.Run(fmt.Sprintf("Test%d", id), func() {

			ctx := context.Background()
			commandCtx, cancel := context.WithTimeout(ctx, time.Second)
			defer cancel()

			responses := map[string][]byte{
				"info": testCase.Payload,
			}
			go test.serviceInstance.Process(commandCtx, responses)

			err := InstanceSanityCheck(commandCtx, b)
			if len(testCase.err) == 0 {
				test.NoError(err)
			} else {
				test.ErrorContains(err, testCase.err)
			}
		})
	}
}

func (test *InstanceTest) TestInstanceStop() {
	test.Run("fail to unmarshal", func() {
		b := new(stream.MockMessage)
		b.On("ParsePayload", &st.InstanceStopCommand{}).Return(errors.New("error unmarshalling"))
		ctx := context.Background()
		commandCtx, cancel := context.WithTimeout(ctx, time.Second)
		defer cancel()

		err := InstanceStop(commandCtx, b)
		test.Error(err)
		test.ErrorContains(err, "error unmarshalling")
	})
	test.Run("fail to stop", func() {
		b := new(stream.MockMessage)
		c := new(container.MockContainer)

		b.On("ParsePayload", &st.InstanceStopCommand{}).Return(nil).Run(func(args mock.Arguments) {
			payload := args.Get(0).(*st.InstanceStopCommand)
			payload.ID = test.testInstance.ID
		})
		b.On("MustGet", serviceDependency.ContainerDep).Return(c)

		c.On("Stop", mock.Anything, test.testInstance.ID, true).Return(errors.New("error stopping instance"))

		ctx := context.Background()
		commandCtx, cancel := context.WithTimeout(ctx, time.Second)
		defer cancel()

		err := InstanceStop(commandCtx, b)
		test.Error(err)
		test.EqualError(err, "error stopping instance")
	})
	test.Run("Successful", func() {
		b := new(stream.MockMessage)
		c := new(container.MockContainer)

		b.On("ParsePayload", &st.InstanceStopCommand{}).Return(nil).Run(func(args mock.Arguments) {
			payload := args.Get(0).(*st.InstanceStopCommand)
			payload.ID = test.testInstance.ID
		})
		b.On("MustGet", serviceDependency.ContainerDep).Return(c)

		c.On("Stop", mock.Anything, test.testInstance.ID, true).Return(nil)

		ctx := context.Background()
		commandCtx, cancel := context.WithTimeout(ctx, time.Second)
		defer cancel()

		err := InstanceStop(commandCtx, b)
		test.NoError(err)
	})
}
