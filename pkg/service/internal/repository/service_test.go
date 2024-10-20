package repository_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/internal/test_helper"
	"github.com/lhjnilsson/foreverbull/pkg/service/internal/repository"
	"github.com/lhjnilsson/foreverbull/pkg/service/pb"
	"github.com/stretchr/testify/suite"
)

type ServiceTest struct {
	suite.Suite

	conn *pgxpool.Pool
}

func (test *ServiceTest) SetupSuite() {
	test_helper.SetupEnvironment(test.T(), &test_helper.Containers{
		Postgres: true,
	})
}

func (test *ServiceTest) SetupTest() {
	var err error
	test.conn, err = pgxpool.New(context.Background(), environment.GetPostgresURL())
	test.Require().NoError(err)

	ctx := context.Background()
	err = repository.Recreate(ctx, test.conn)
	test.Require().NoError(err)
}

func (test *ServiceTest) TearDownTest() {
}

func TestServices(t *testing.T) {
	suite.Run(t, new(ServiceTest))
}

func (test *ServiceTest) TestCreate() {
	ctx := context.Background()

	services := &repository.Service{Conn: test.conn}
	service, err := services.Create(ctx, "test_image")
	test.Require().NoError(err)
	test.Equal("test_image", service.Image)
	test.Len(service.Statuses, 1)
	test.Equal(pb.Service_Status_CREATED.String(), service.Statuses[0].Status.String())
}

func (test *ServiceTest) TestGet() {
	ctx := context.Background()

	services := &repository.Service{Conn: test.conn}
	_, err := services.Create(ctx, "test_image")
	test.Require().NoError(err)

	service, err := services.Get(ctx, "test_image")
	test.Require().NoError(err)
	test.Equal("test_image", service.Image)
	test.Len(service.Statuses, 1)
	test.Equal(pb.Service_Status_CREATED.String(), service.Statuses[0].Status.String())
}

func (test *ServiceTest) TestSetAlgorithm() {
	ctx := context.Background()

	services := &repository.Service{Conn: test.conn}
	_, err := services.Create(ctx, "image")
	test.Require().NoError(err)

	algorithm := &pb.Algorithm{
		FilePath: "/file.py",
	}
	err = services.SetAlgorithm(ctx, "image", algorithm)
	test.Require().NoError(err)

	service, err := services.Get(ctx, "image")
	test.Require().NoError(err)
	test.Equal("image", service.Image)
	test.NotNil(service.Algorithm)
	test.Equal("/file.py", service.Algorithm.FilePath)
}

func (test *ServiceTest) TestUpdateStatus() {
	ctx := context.Background()

	services := &repository.Service{Conn: test.conn}
	_, err := services.Create(ctx, "image")
	test.Require().NoError(err)

	err = services.UpdateStatus(ctx, "image", pb.Service_Status_READY, nil)
	test.Require().NoError(err)

	service, err := services.Get(ctx, "image")
	test.Require().NoError(err)
	test.Equal("image", service.Image)
	test.Len(service.Statuses, 2)
	test.Equal(pb.Service_Status_READY.String(), service.Statuses[0].Status.String())
	test.Equal(pb.Service_Status_CREATED.String(), service.Statuses[1].Status.String())
}

func (test *ServiceTest) TestList() {
	ctx := context.Background()

	services := &repository.Service{Conn: test.conn}
	_, err := services.Create(ctx, "image1")
	test.Require().NoError(err)
	_, err = services.Create(ctx, "image2")
	test.Require().NoError(err)

	storedServices, err := services.List(ctx)
	test.Require().NoError(err)
	test.Len(storedServices, 2)
}

func (test *ServiceTest) TestDelete() {
	ctx := context.Background()

	services := &repository.Service{Conn: test.conn}
	_, err := services.Create(ctx, "image")
	test.Require().NoError(err)

	err = services.Delete(ctx, "image")
	test.Require().NoError(err)

	service, err := services.Get(ctx, "image")
	test.Error(err)
	test.Nil(service)
}
