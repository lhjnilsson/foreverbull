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

	db := &repository.Service{Conn: test.conn}
	service, err := db.Create(ctx, "test_image")
	test.NoError(err)
	test.Equal("test_image", service.Image)
	test.Len(service.Statuses, 1)
	test.Equal(pb.Service_Status_CREATED.String(), service.Statuses[0].Status.String())
}

func (test *ServiceTest) TestGet() {
	ctx := context.Background()

	db := &repository.Service{Conn: test.conn}
	_, err := db.Create(ctx, "test_image")
	test.NoError(err)

	service, err := db.Get(ctx, "test_image")
	test.NoError(err)
	test.Equal("test_image", service.Image)
	test.Len(service.Statuses, 1)
	test.Equal(pb.Service_Status_CREATED.String(), service.Statuses[0].Status.String())
}

func (test *ServiceTest) TestSetAlgorithm() {
	ctx := context.Background()

	db := &repository.Service{Conn: test.conn}
	_, err := db.Create(ctx, "image")
	test.NoError(err)

	algorithm := &pb.Algorithm{
		FilePath: "/file.py",
	}
	err = db.SetAlgorithm(ctx, "image", algorithm)
	test.NoError(err)

	service, err := db.Get(ctx, "image")
	test.NoError(err)
	test.Equal("image", service.Image)
	test.NotNil(service.Algorithm)
	test.Equal("/file.py", service.Algorithm.FilePath)
}

func (test *ServiceTest) TestUpdateStatus() {
	ctx := context.Background()

	db := &repository.Service{Conn: test.conn}
	_, err := db.Create(ctx, "image")
	test.NoError(err)

	err = db.UpdateStatus(ctx, "image", pb.Service_Status_READY, nil)
	test.NoError(err)

	service, err := db.Get(ctx, "image")
	test.NoError(err)
	test.Equal("image", service.Image)
	test.Len(service.Statuses, 2)
	test.Equal(pb.Service_Status_READY.String(), service.Statuses[0].Status.String())
	test.Equal(pb.Service_Status_CREATED.String(), service.Statuses[1].Status.String())
}

func (test *ServiceTest) TestList() {
	ctx := context.Background()

	db := &repository.Service{Conn: test.conn}
	_, err := db.Create(ctx, "image1")
	test.NoError(err)
	_, err = db.Create(ctx, "image2")
	test.NoError(err)

	services, err := db.List(ctx)
	test.NoError(err)
	test.Len(services, 2)
}

func (test *ServiceTest) TestDelete() {
	ctx := context.Background()

	db := &repository.Service{Conn: test.conn}
	_, err := db.Create(ctx, "image")
	test.NoError(err)

	err = db.Delete(ctx, "image")
	test.NoError(err)

	service, err := db.Get(ctx, "image")
	test.Error(err)
	test.Nil(service)
}
