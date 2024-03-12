package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/service/entity"
	"github.com/lhjnilsson/foreverbull/tests/helper"
	"github.com/stretchr/testify/suite"
)

type InstanceTest struct {
	suite.Suite

	conn *pgxpool.Pool
}

func (test *InstanceTest) SetupTest() {
	var err error

	helper.SetupEnvironment(test.T(), &helper.Containers{
		Postgres: true,
	})
	test.conn, err = pgxpool.New(context.Background(), environment.GetPostgresURL())
	test.Require().NoError(err)
	ctx := context.Background()

	err = Recreate(ctx, test.conn)
	test.Require().NoError(err)

	s_repository := &Service{Conn: test.conn}
	_, err = s_repository.Create(ctx, "image")
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

	db := &Instance{Conn: test.conn}
	instance, err := db.Create(ctx, "instance", "image")
	test.NoError(err)
	test.Equal("instance", instance.ID)
}

func (test *InstanceTest) TestGet() {
	ctx := context.Background()

	db := &Instance{Conn: test.conn}
	_, err := db.Create(ctx, "instance", "image")
	test.NoError(err)

	instance, err := db.Get(ctx, "instance")
	test.NoError(err)
	test.Equal("instance", instance.ID)
}

func (test *InstanceTest) TestUpdateHostPort() {
	ctx := context.Background()

	db := &Instance{Conn: test.conn}
	_, err := db.Create(ctx, "instance", "image")
	test.NoError(err)

	err = db.UpdateHostPort(ctx, "instance", "host", 1234)
	test.NoError(err)

	instance, err := db.Get(ctx, "instance")
	test.NoError(err)
	test.Equal("host", *instance.Host)
	test.Equal(1234, *instance.Port)
}

func (test *InstanceTest) TestUpdateStatus() {
	ctx := context.Background()

	db := &Instance{Conn: test.conn}
	_, err := db.Create(ctx, "instance", "image")
	test.NoError(err)

	err = db.UpdateStatus(ctx, "instance", entity.InstanceStatusRunning, nil)
	test.NoError(err)

	err = db.UpdateStatus(ctx, "instance", entity.InstanceStatusError, errors.New("test_error"))
	test.NoError(err)

	instance, err := db.Get(ctx, "instance")
	test.NoError(err)
	test.Len(instance.Statuses, 3)
	test.Equal(entity.InstanceStatusError, instance.Statuses[0].Status)
	test.Equal("test_error", *instance.Statuses[0].Error)
	test.Equal(entity.InstanceStatusRunning, instance.Statuses[1].Status)
	test.Nil(instance.Statuses[1].Error)
	test.Equal(entity.InstanceStatusCreated, instance.Statuses[2].Status)
	test.Nil(instance.Statuses[2].Error)
}

func (test *InstanceTest) TestList() {
	ctx := context.Background()

	db := &Instance{Conn: test.conn}
	_, err := db.Create(ctx, "instance1", "image")
	test.NoError(err)
	err = db.UpdateStatus(ctx, "instance1", entity.InstanceStatusRunning, nil)
	test.NoError(err)

	_, err = db.Create(ctx, "instance2", "image")
	test.NoError(err)
	err = db.UpdateStatus(ctx, "instance2", entity.InstanceStatusError, errors.New("test_error"))
	test.NoError(err)

	instances, err := db.List(ctx)
	test.NoError(err)
	test.Len(*instances, 2)

	instance2 := (*instances)[0]
	test.Equal("instance2", instance2.ID)
	test.Equal("image", instance2.Image)
	test.Len(instance2.Statuses, 2)
	test.Equal(entity.InstanceStatusError, instance2.Statuses[0].Status)
	test.Equal("test_error", *instance2.Statuses[0].Error)

	instance1 := (*instances)[1]
	test.Equal("instance1", instance1.ID)
	test.Equal("image", instance1.Image)
	test.Len(instance1.Statuses, 2)
	test.Equal(entity.InstanceStatusRunning, instance1.Statuses[0].Status)
	test.Nil(instance1.Statuses[0].Error)
}

func (test *InstanceTest) TestListByImage() {
	ctx := context.Background()

	db := &Instance{Conn: test.conn}
	_, err := db.Create(ctx, "instance1", "image")
	test.NoError(err)
	err = db.UpdateStatus(ctx, "instance1", entity.InstanceStatusRunning, nil)
	test.NoError(err)

	instances, err := db.ListByImage(ctx, "image")
	test.NoError(err)
	test.Require().Len(*instances, 1)

	instance1 := (*instances)[0]
	test.Equal("instance1", instance1.ID)
	test.Equal("image", instance1.Image)
	test.Len(instance1.Statuses, 2)
	test.Equal(entity.InstanceStatusRunning, instance1.Statuses[0].Status)
	test.Nil(instance1.Statuses[0].Error)

	instances, err = db.ListByImage(ctx, "image2")
	test.NoError(err)
	test.Len(*instances, 0)
}
