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

	s_repository := &repository.Service{Conn: test.conn}
	_, err = s_repository.Create(ctx, "test_image")
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
		db := &repository.Instance{Conn: test.conn}
		instance, err := db.Create(ctx, fmt.Sprintf("instance_%d", index), image)
		test.NoError(err)
		test.Equal(fmt.Sprintf("instance_%d", index), instance.ID)
		test.Equal(image, instance.Image)
	}
}

func (test *InstanceTest) TestGet() {
	ctx := context.Background()

	image := "test_image"
	for index, image := range []*string{nil, &image} {
		db := &repository.Instance{Conn: test.conn}
		_, err := db.Create(ctx, fmt.Sprintf("instance_%d", index), image)
		test.NoError(err)

		instance, err := db.Get(ctx, fmt.Sprintf("instance_%d", index))
		test.NoError(err)
		test.Equal(fmt.Sprintf("instance_%d", index), instance.ID)
		test.Equal(image, instance.Image)
	}
}

func (test *InstanceTest) TestUpdateHostPort() {
	ctx := context.Background()

	db := &repository.Instance{Conn: test.conn}
	_, err := db.Create(ctx, "instance", nil)
	test.NoError(err)

	err = db.UpdateHostPort(ctx, "instance", "host", 1234)
	test.NoError(err)

	instance, err := db.Get(ctx, "instance")
	test.NoError(err)
	test.Require().NotNil(instance.Host)
	test.Equal("host", *instance.Host)
	test.Require().NotNil(instance.Port)
	test.Equal(int32(1234), *instance.Port)
}

func (test *InstanceTest) TestUpdateStatus() {
	ctx := context.Background()

	db := &repository.Instance{Conn: test.conn}
	_, err := db.Create(ctx, "instance", nil)
	test.NoError(err)

	err = db.UpdateStatus(ctx, "instance", pb.Instance_Status_RUNNING, nil)
	test.NoError(err)

	instance, err := db.Get(ctx, "instance")
	test.NoError(err)
	test.Equal(pb.Instance_Status_RUNNING.String(), instance.Statuses[0].Status.String())
}

func (test *InstanceTest) TestList() {
	ctx := context.Background()

	db := &repository.Instance{Conn: test.conn}
	_, err := db.Create(ctx, "instance1", nil)
	test.NoError(err)
	_, err = db.Create(ctx, "instance2", nil)
	test.NoError(err)

	instances, err := db.List(ctx)
	test.NoError(err)
	test.Len(instances, 2)
}

func (test *InstanceTest) TestListByImage() {
	ctx := context.Background()

	image := "test_image"

	db := &repository.Instance{Conn: test.conn}
	_, err := db.Create(ctx, "instance1", &image)
	test.NoError(err)
	_, err = db.Create(ctx, "instance2", nil)
	test.NoError(err)

	instances, err := db.ListByImage(ctx, image)
	test.NoError(err)
	test.Len(instances, 1)
}
