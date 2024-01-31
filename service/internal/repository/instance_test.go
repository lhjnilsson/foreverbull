package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/service/entity"
	"github.com/lhjnilsson/foreverbull/tests/helper"
	"github.com/stretchr/testify/suite"
)

type InstanceTest struct {
	suite.Suite

	conn *pgxpool.Pool
}

func (suite *InstanceTest) SetupTest() {
	var err error

	config := helper.TestingConfig(suite.T(), &helper.Containers{
		Postgres: true,
	})
	suite.conn, err = pgxpool.New(context.Background(), config.PostgresURI)
	suite.NoError(err)
	ctx := context.Background()

	err = Recreate(ctx, suite.conn)
	suite.NoError(err)

	s_repository := &Service{Conn: suite.conn}
	_, err = s_repository.Create(ctx, "service", "image")
	suite.NoError(err)
	err = s_repository.UpdateServiceInfo(ctx, "service", "type", nil)
	suite.NoError(err)
}

func (suite *InstanceTest) TearDownTest() {
}

func TestInstances(t *testing.T) {
	suite.Run(t, new(InstanceTest))
}

func (suite *InstanceTest) TestCreate() {
	ctx := context.Background()

	db := &Instance{Conn: suite.conn}
	instance, err := db.Create(ctx, "instance", "service")
	suite.NoError(err)
	suite.Equal("instance", instance.ID)
}

func (suite *InstanceTest) TestGet() {
	ctx := context.Background()

	db := &Instance{Conn: suite.conn}
	_, err := db.Create(ctx, "instance", "service")
	suite.NoError(err)

	instance, err := db.Get(ctx, "instance")
	suite.NoError(err)
	suite.Equal("instance", instance.ID)
}

func (suite *InstanceTest) TestUpdateHostPort() {
	ctx := context.Background()

	db := &Instance{Conn: suite.conn}
	_, err := db.Create(ctx, "instance", "service")
	suite.NoError(err)

	err = db.UpdateHostPort(ctx, "instance", "host", 1234)
	suite.NoError(err)

	instance, err := db.Get(ctx, "instance")
	suite.NoError(err)
	suite.Equal("host", *instance.Host)
	suite.Equal(1234, *instance.Port)
}

func (suite *InstanceTest) TestUpdateStatus() {
	ctx := context.Background()

	db := &Instance{Conn: suite.conn}
	_, err := db.Create(ctx, "instance", "service")
	suite.NoError(err)

	err = db.UpdateStatus(ctx, "instance", entity.InstanceStatusRunning, nil)
	suite.NoError(err)

	err = db.UpdateStatus(ctx, "instance", entity.InstanceStatusError, errors.New("test_error"))
	suite.NoError(err)

	instance, err := db.Get(ctx, "instance")
	suite.NoError(err)
	suite.Len(instance.Statuses, 3)
	suite.Equal(entity.InstanceStatusError, instance.Statuses[0].Status)
	suite.Equal("test_error", *instance.Statuses[0].Error)
	suite.Equal(entity.InstanceStatusRunning, instance.Statuses[1].Status)
	suite.Nil(instance.Statuses[1].Error)
	suite.Equal(entity.InstanceStatusCreated, instance.Statuses[2].Status)
	suite.Nil(instance.Statuses[2].Error)
}

func (suite *InstanceTest) TestList() {
	ctx := context.Background()

	db := &Instance{Conn: suite.conn}
	_, err := db.Create(ctx, "instance1", "service")
	suite.NoError(err)
	err = db.UpdateStatus(ctx, "instance1", entity.InstanceStatusRunning, nil)
	suite.NoError(err)

	_, err = db.Create(ctx, "instance2", "service")
	suite.NoError(err)
	err = db.UpdateStatus(ctx, "instance2", entity.InstanceStatusError, errors.New("test_error"))
	suite.NoError(err)

	instances, err := db.List(ctx)
	suite.NoError(err)
	suite.Len(*instances, 2)

	instance2 := (*instances)[0]
	suite.Equal("instance2", instance2.ID)
	suite.Equal("service", instance2.Service)
	suite.Len(instance2.Statuses, 2)
	suite.Equal(entity.InstanceStatusError, instance2.Statuses[0].Status)
	suite.Equal("test_error", *instance2.Statuses[0].Error)

	instance1 := (*instances)[1]
	suite.Equal("instance1", instance1.ID)
	suite.Equal("service", instance1.Service)
	suite.Len(instance1.Statuses, 2)
	suite.Equal(entity.InstanceStatusRunning, instance1.Statuses[0].Status)
	suite.Nil(instance1.Statuses[0].Error)
}

func (suite *InstanceTest) TestListByService() {
	ctx := context.Background()

	db := &Instance{Conn: suite.conn}
	_, err := db.Create(ctx, "instance1", "service")
	suite.NoError(err)
	err = db.UpdateStatus(ctx, "instance1", entity.InstanceStatusRunning, nil)
	suite.NoError(err)

	instances, err := db.ListByService(ctx, "service")
	suite.NoError(err)
	suite.Len(*instances, 1)

	instance1 := (*instances)[0]
	suite.Equal("instance1", instance1.ID)
	suite.Equal("service", instance1.Service)
	suite.Len(instance1.Statuses, 2)
	suite.Equal(entity.InstanceStatusRunning, instance1.Statuses[0].Status)
	suite.Nil(instance1.Statuses[0].Error)

	instances, err = db.ListByService(ctx, "service2")
	suite.NoError(err)
	suite.Len(*instances, 0)
}
