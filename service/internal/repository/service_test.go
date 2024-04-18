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

type ServiceTest struct {
	suite.Suite

	conn *pgxpool.Pool
}

func (test *ServiceTest) SetupTest() {
	var err error
	helper.SetupEnvironment(test.T(), &helper.Containers{
		Postgres: true,
	})
	test.conn, err = pgxpool.New(context.Background(), environment.GetPostgresURL())
	test.Require().NoError(err)
	ctx := context.Background()
	err = Recreate(ctx, test.conn)
	test.Require().NoError(err)
}

func (test *ServiceTest) TearDownTest() {
}

func TestServices(t *testing.T) {
	suite.Run(t, new(ServiceTest))
}

func (test *ServiceTest) TestCreate() {
	ctx := context.Background()

	db := &Service{Conn: test.conn}
	service, err := db.Create(ctx, "image")
	test.NoError(err)
	test.Equal("image", service.Image)
	test.Len(service.Statuses, 1)
	test.Equal(entity.ServiceStatusCreated, service.Statuses[0].Status)
}

func (test *ServiceTest) TestGet() {
	ctx := context.Background()

	db := &Service{Conn: test.conn}
	_, err := db.Create(ctx, "image")
	test.NoError(err)

	service, err := db.Get(ctx, "image")
	test.NoError(err)
	test.Equal("image", service.Image)
	test.Len(service.Statuses, 1)
	test.Equal(entity.ServiceStatusCreated, service.Statuses[0].Status)
}

func (test *ServiceTest) TestUpdateServiceAlgorithm() {
	ctx := context.Background()

	db := &Service{Conn: test.conn}
	_, err := db.Create(ctx, "image")
	test.NoError(err)

	algorithm := entity.ServiceAlgorithm{}
	err = db.UpdateAlgorithm(ctx, "image", &algorithm)
	test.NoError(err)

	service, err := db.Get(ctx, "image")
	test.NoError(err)
	test.Equal("image", service.Image)
	test.Len(service.Statuses, 1)
	test.Equal(entity.ServiceStatusCreated, service.Statuses[0].Status)
	test.Equal(algorithm, *service.Algorithm)
}

func (test *ServiceTest) TestUpdateStatus() {
	ctx := context.Background()

	db := &Service{Conn: test.conn}
	_, err := db.Create(ctx, "image")
	test.NoError(err)

	err = db.UpdateStatus(ctx, "image", entity.ServiceStatusReady, nil)
	test.NoError(err)

	err = db.UpdateStatus(ctx, "image", entity.ServiceStatusError, errors.New("test_error"))
	test.NoError(err)

	service, err := db.Get(ctx, "image")
	test.NoError(err)
	test.Equal("image", service.Image)
	test.Len(service.Statuses, 3)
	test.Equal(entity.ServiceStatusError, service.Statuses[0].Status)
	test.Equal("test_error", *service.Statuses[0].Error)
	test.Equal(entity.ServiceStatusReady, service.Statuses[1].Status)
	test.Equal(entity.ServiceStatusCreated, service.Statuses[2].Status)
}

func (test *ServiceTest) TestList() {
	ctx := context.Background()

	db := &Service{Conn: test.conn}
	_, err := db.Create(ctx, "image1")
	test.NoError(err)
	err = db.UpdateStatus(ctx, "image1", entity.ServiceStatusReady, nil)
	test.NoError(err)

	_, err = db.Create(ctx, "image2")
	test.NoError(err)
	err = db.UpdateStatus(ctx, "image2", entity.ServiceStatusError, errors.New("test_error"))
	test.NoError(err)

	services, err := db.List(ctx)
	test.NoError(err)
	test.Len(*services, 2)

	service2 := (*services)[0]
	test.Equal("image2", service2.Image)
	test.Len(service2.Statuses, 2)
	test.Equal(entity.ServiceStatusError, service2.Statuses[0].Status)
	test.Equal("test_error", *service2.Statuses[0].Error)

	service1 := (*services)[1]
	test.Equal("image1", service1.Image)
	test.Len(service1.Statuses, 2)
	test.Equal(entity.ServiceStatusReady, service1.Statuses[0].Status)
}

func (test *ServiceTest) TestDelete() {
	ctx := context.Background()

	db := &Service{Conn: test.conn}
	_, err := db.Create(ctx, "image")
	test.NoError(err)

	err = db.Delete(ctx, "image")
	test.NoError(err)

	services, err := db.List(ctx)
	test.Nil(err)
	test.Empty(*services)
}
