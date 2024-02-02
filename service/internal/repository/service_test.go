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

func (suite *ServiceTest) SetupTest() {
	var err error
	helper.SetupEnvironment(suite.T(), &helper.Containers{
		Postgres: true,
	})
	suite.conn, err = pgxpool.New(context.Background(), environment.GetPostgresURL())
	suite.NoError(err)
	ctx := context.Background()
	err = Recreate(ctx, suite.conn)
	suite.NoError(err)
}

func (suite *ServiceTest) TearDownTest() {
}

func TestServices(t *testing.T) {
	suite.Run(t, new(ServiceTest))
}

func (suite *ServiceTest) TestCreate() {
	ctx := context.Background()

	db := &Service{Conn: suite.conn}
	service, err := db.Create(ctx, "service", "image")
	suite.NoError(err)
	suite.Equal("service", service.Name)
	suite.Equal("image", service.Image)
	suite.Len(service.Statuses, 1)
	suite.Equal(entity.ServiceStatusCreated, service.Statuses[0].Status)
}

func (suite *ServiceTest) TestGet() {
	ctx := context.Background()

	db := &Service{Conn: suite.conn}
	_, err := db.Create(ctx, "service", "image")
	suite.NoError(err)

	service, err := db.Get(ctx, "service")
	suite.NoError(err)
	suite.Equal("service", service.Name)
	suite.Equal("image", service.Image)
	suite.Len(service.Statuses, 1)
	suite.Equal(entity.ServiceStatusCreated, service.Statuses[0].Status)
}

func (suite *ServiceTest) TestUpdateServiceInfo() {
	ctx := context.Background()

	db := &Service{Conn: suite.conn}
	_, err := db.Create(ctx, "service", "image")
	suite.NoError(err)

	parameters := []entity.Parameter{
		{
			Key:   "key1",
			Type:  "int",
			Value: "22",
		},
	}

	err = db.UpdateServiceInfo(ctx, "service", "service_type", &parameters)
	suite.NoError(err)

	service, err := db.Get(ctx, "service")
	suite.NoError(err)
	suite.Equal("service", service.Name)
	suite.Equal("image", service.Image)
	suite.Len(service.Statuses, 1)
	suite.Equal(entity.ServiceStatusCreated, service.Statuses[0].Status)
	suite.Equal("service_type", *service.Type)
	suite.Len(*service.WorkerParameters, 1)
}

func (suite *ServiceTest) TestUpdateStatus() {
	ctx := context.Background()

	db := &Service{Conn: suite.conn}
	_, err := db.Create(ctx, "service", "image")
	suite.NoError(err)

	err = db.UpdateStatus(ctx, "service", entity.ServiceStatusReady, nil)
	suite.NoError(err)

	err = db.UpdateStatus(ctx, "service", entity.ServiceStatusError, errors.New("test_error"))
	suite.NoError(err)

	service, err := db.Get(ctx, "service")
	suite.NoError(err)
	suite.Equal("service", service.Name)
	suite.Equal("image", service.Image)
	suite.Len(service.Statuses, 3)
	suite.Equal(entity.ServiceStatusError, service.Statuses[0].Status)
	suite.Equal("test_error", *service.Statuses[0].Error)
	suite.Equal(entity.ServiceStatusReady, service.Statuses[1].Status)
	suite.Equal(entity.ServiceStatusCreated, service.Statuses[2].Status)
}

func (suite *ServiceTest) TestList() {
	ctx := context.Background()

	db := &Service{Conn: suite.conn}
	_, err := db.Create(ctx, "service1", "image")
	suite.NoError(err)
	err = db.UpdateStatus(ctx, "service1", entity.ServiceStatusReady, nil)
	suite.NoError(err)

	_, err = db.Create(ctx, "service2", "image")
	suite.NoError(err)
	err = db.UpdateStatus(ctx, "service2", entity.ServiceStatusError, errors.New("test_error"))
	suite.NoError(err)

	services, err := db.List(ctx)
	suite.NoError(err)
	suite.Len(*services, 2)

	service2 := (*services)[0]
	suite.Equal("service2", service2.Name)
	suite.Equal("image", service2.Image)
	suite.Len(service2.Statuses, 2)
	suite.Equal(entity.ServiceStatusError, service2.Statuses[0].Status)
	suite.Equal("test_error", *service2.Statuses[0].Error)

	service1 := (*services)[1]
	suite.Equal("service1", service1.Name)
	suite.Equal("image", service1.Image)
	suite.Len(service1.Statuses, 2)
	suite.Equal(entity.ServiceStatusReady, service1.Statuses[0].Status)
}

func (suite *ServiceTest) TestDelete() {
	ctx := context.Background()

	db := &Service{Conn: suite.conn}
	_, err := db.Create(ctx, "service", "image")
	suite.NoError(err)

	err = db.Delete(ctx, "service")
	suite.NoError(err)

	services, err := db.List(ctx)
	suite.Nil(err)
	suite.Empty(*services)
}
