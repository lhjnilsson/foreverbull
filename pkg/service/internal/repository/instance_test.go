package repository_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/internal/test_helper"
	"github.com/lhjnilsson/foreverbull/pkg/service/internal/repository"
	"github.com/lhjnilsson/foreverbull/pkg/service/pb"
	"github.com/stretchr/testify/suite"
)

type InstanceTest struct {
	suite.Suite

	conn *pgxpool.Pool
}

func (test *InstanceTest) SetupSuite() {
	test_helper.SetupEnvironment(test.T(), &test_helper.Containers{
		Postgres: true,
	})
}

func (test *InstanceTest) SetupTest() {
	var err error
	test.conn, err = pgxpool.New(context.Background(), environment.GetPostgresURL())
	test.Require().NoError(err)

	ctx := context.Background()

	err = repository.Recreate(ctx, test.conn)
	test.Require().NoError(err)

	services := &repository.Service{Conn: test.conn}
	_, err = services.Create(ctx, "test_image")
	test.Require().NoError(err)
	test.Require().NoError(err)
}

func (test *InstanceTest) TearDownTest() {
}

func TestInstances(t *testing.T) {
	suite.Run(t, new(InstanceTest))
}

func (test *InstanceTest) TestCreate() {
	ctx := context.Background()

	image := "test_image"
	for index, image := range []*string{nil, &image} {
		instances := &repository.Instance{Conn: test.conn}
		instance, err := instances.Create(ctx, fmt.Sprintf("instance_%d", index), image)
		test.Require().NoError(err)
		test.Equal(fmt.Sprintf("instance_%d", index), instance.ID)
		test.Equal(image, instance.Image)
	}
}

func (test *InstanceTest) TestGet() {
	ctx := context.Background()

	image := "test_image"
	for index, image := range []*string{nil, &image} {
		instances := &repository.Instance{Conn: test.conn}
		_, err := instances.Create(ctx, fmt.Sprintf("instance_%d", index), image)
		test.Require().NoError(err)

		instance, err := instances.Get(ctx, fmt.Sprintf("instance_%d", index))
		test.Require().NoError(err)
		test.Equal(fmt.Sprintf("instance_%d", index), instance.ID)
		test.Equal(image, instance.Image)
	}
}

func (test *InstanceTest) TestUpdateHostPort() {
	ctx := context.Background()

	instances := &repository.Instance{Conn: test.conn}
	_, err := instances.Create(ctx, "instance", nil)
	test.Require().NoError(err)

	err = instances.UpdateHostPort(ctx, "instance", "host", 1234)
	test.Require().NoError(err)

	instance, err := instances.Get(ctx, "instance")
	test.Require().NoError(err)
	test.Require().NotNil(instance.Host)
	test.Equal("host", *instance.Host)
	test.Require().NotNil(instance.Port)
	test.Equal(int32(1234), *instance.Port)
}

func (test *InstanceTest) TestUpdateStatus() {
	ctx := context.Background()

	instances := &repository.Instance{Conn: test.conn}
	_, err := instances.Create(ctx, "instance", nil)
	test.Require().NoError(err)

	err = instances.UpdateStatus(ctx, "instance", pb.Instance_Status_RUNNING, nil)
	test.Require().NoError(err)

	instance, err := instances.Get(ctx, "instance")
	test.Require().NoError(err)
	test.Equal(pb.Instance_Status_RUNNING.String(), instance.Statuses[1].Status.String())
}

func (test *InstanceTest) TestList() {
	ctx := context.Background()

	instances := &repository.Instance{Conn: test.conn}
	_, err := instances.Create(ctx, "instance1", nil)
	test.Require().NoError(err)
	_, err = instances.Create(ctx, "instance2", nil)
	test.Require().NoError(err)

	storedInstances, err := instances.List(ctx)
	test.Require().NoError(err)
	test.Len(storedInstances, 2)
}

func (test *InstanceTest) TestListByImage() {
	ctx := context.Background()

	image := "test_image"

	instances := &repository.Instance{Conn: test.conn}
	_, err := instances.Create(ctx, "instance1", &image)
	test.Require().NoError(err)
	_, err = instances.Create(ctx, "instance2", nil)
	test.Require().NoError(err)

	storedInstances, err := instances.ListByImage(ctx, image)
	test.Require().NoError(err)
	test.Len(storedInstances, 1)
}
