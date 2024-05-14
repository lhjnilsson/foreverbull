package repository

import (
	"context"
	"fmt"
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

func (test *InstanceTest) SetupSuite() {
	helper.SetupEnvironment(test.T(), &helper.Containers{
		Postgres: true,
	})
}

func (test *InstanceTest) SetupTest() {
	var err error
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

	image := "test_image"
	for id, image := range []*string{nil, &image} {
		db := &Instance{Conn: test.conn}
		instance, err := db.Create(ctx, fmt.Sprintf("instance_%d", id), image)
		test.NoError(err)
		test.Equal(fmt.Sprintf("instance_%d", id), instance.ID)
		test.Equal(image, instance.Image)
	}
}

func (test *InstanceTest) TestGet() {
	ctx := context.Background()

	image := "test_image"
	for id, image := range []*string{nil, &image} {
		db := &Instance{Conn: test.conn}
		_, err := db.Create(ctx, fmt.Sprintf("instance_%d", id), image)
		test.NoError(err)

		instance, err := db.Get(ctx, fmt.Sprintf("instance_%d", id))
		test.NoError(err)
		test.Equal(fmt.Sprintf("instance_%d", id), instance.ID)
		test.Equal(image, instance.Image)
	}
}

func (test *InstanceTest) TestUpdateHostPort() {
	ctx := context.Background()

	db := &Instance{Conn: test.conn}
	_, err := db.Create(ctx, "instance", nil)
	test.NoError(err)

	err = db.UpdateHostPort(ctx, "instance", "host", 1234)
	test.NoError(err)

	instance, err := db.Get(ctx, "instance")
	test.NoError(err)
	test.Require().NotNil(instance.Host)
	test.Equal("host", *instance.Host)
	test.Require().NotNil(instance.Port)
	test.Equal(1234, *instance.Port)
}

func (test *InstanceTest) TestUpdateBrokerPort() {
	ctx := context.Background()

	db := &Instance{Conn: test.conn}
	_, err := db.Create(ctx, "instance", nil)
	test.NoError(err)

	err = db.UpdateBrokerPort(ctx, "instance", 1234)
	test.NoError(err)

	instance, err := db.Get(ctx, "instance")
	test.NoError(err)
	test.Require().NotNil(instance.BrokerPort)
	test.Equal(1234, *instance.BrokerPort)
}

func (test *InstanceTest) TestUpdateNamespacePort() {
	ctx := context.Background()

	db := &Instance{Conn: test.conn}
	_, err := db.Create(ctx, "instance", nil)
	test.NoError(err)

	err = db.UpdateNamespacePort(ctx, "instance", 1234)
	test.NoError(err)

	instance, err := db.Get(ctx, "instance")
	test.NoError(err)
	test.Require().NotNil(instance.NamespacePort)
	test.Equal(1234, *instance.NamespacePort)
}

func (test *InstanceTest) TestUpdateDatabaseURL() {
	ctx := context.Background()

	db := &Instance{Conn: test.conn}
	_, err := db.Create(ctx, "instance", nil)
	test.NoError(err)

	err = db.UpdateDatabaseURL(ctx, "instance", "database_url")
	test.NoError(err)

	instance, err := db.Get(ctx, "instance")
	test.NoError(err)
	test.Require().NotNil(instance.DatabaseURL)
	test.Equal("database_url", *instance.DatabaseURL)
}

func (test *InstanceTest) TestUpdateFunctions() {
	ctx := context.Background()

	db := &Instance{Conn: test.conn}
	_, err := db.Create(ctx, "instance", nil)
	test.NoError(err)

	functions := map[string]entity.InstanceFunction{
		"function": {
			Parameters: map[string]string{
				"param": "value",
			},
		},
	}
	err = db.UpdateFunctions(ctx, "instance", &functions)
	test.NoError(err)

	instance, err := db.Get(ctx, "instance")
	test.NoError(err)
	test.Require().NotNil(instance.Functions)
	test.Equal(functions, *instance.Functions)
}

func (test *InstanceTest) TestUpdateStatus() {
	ctx := context.Background()

	db := &Instance{Conn: test.conn}
	_, err := db.Create(ctx, "instance", nil)
	test.NoError(err)

	err = db.UpdateStatus(ctx, "instance", entity.InstanceStatusRunning, nil)
	test.NoError(err)

	instance, err := db.Get(ctx, "instance")
	test.NoError(err)
	test.Equal(entity.InstanceStatusRunning, instance.Statuses[0].Status)
}

func (test *InstanceTest) TestList() {
	ctx := context.Background()

	db := &Instance{Conn: test.conn}
	_, err := db.Create(ctx, "instance1", nil)
	test.NoError(err)
	_, err = db.Create(ctx, "instance2", nil)
	test.NoError(err)

	instances, err := db.List(ctx)
	test.NoError(err)
	test.Len(*instances, 2)
}

func (test *InstanceTest) TestListByImage() {
	ctx := context.Background()

	image := "test_image"

	db := &Instance{Conn: test.conn}
	_, err := db.Create(ctx, "instance1", &image)
	test.NoError(err)
	_, err = db.Create(ctx, "instance2", nil)
	test.NoError(err)

	instances, err := db.ListByImage(ctx, image)
	test.NoError(err)
	test.Len(*instances, 1)
}
