package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/internal/test_helper"
	pb "github.com/lhjnilsson/foreverbull/pkg/pb/service"
	"github.com/lhjnilsson/foreverbull/pkg/service/internal/repository"
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
	test.Require().Error(err)
	test.Nil(service)
}

func (test *ServiceTest) TestServiceStatusPrimaryKey() {
	ctx := context.Background()
	test.Require().NoError(repository.MigrateServiceStatus(ctx, test.conn))
	test.Require().NoError(repository.MigrateServiceStatus(ctx, test.conn))
	requireIdentityPrimaryKey(&test.Suite, test.conn, "service_status")

	acquired, err := test.conn.Acquire(ctx)
	test.Require().NoError(err)
	defer acquired.Release()

	_, err = acquired.Exec(ctx, `LISTEN service_status`)
	test.Require().NoError(err)
	defer func() {
		_, _ = acquired.Exec(context.Background(), `UNLISTEN service_status`)
	}()

	services := &repository.Service{Conn: test.conn}
	_, err = services.Create(ctx, "test_image")
	test.Require().NoError(err)

	notifyCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	notification, err := acquired.Conn().WaitForNotification(notifyCtx)
	test.Require().NoError(err)
	test.Equal("service_status", notification.Channel)
	test.Equal("test_image", notification.Payload)

	var n int
	err = test.conn.QueryRow(ctx, `SELECT COUNT(*) FROM service_status WHERE image = $1`, "test_image").Scan(&n)
	test.Require().NoError(err)
	test.Equal(1, n)

	test.Require().NoError(repository.MigrateServiceStatus(ctx, test.conn))
	err = test.conn.QueryRow(ctx, `SELECT COUNT(*) FROM service_status WHERE image = $1`, "test_image").Scan(&n)
	test.Require().NoError(err)
	test.Equal(1, n)

	var definition string
	err = test.conn.QueryRow(ctx, `SELECT pg_get_functiondef('notify_service_status()'::regprocedure)`).Scan(&definition)
	test.Require().NoError(err)
	test.Contains(definition, "INSERT INTO service_status (image, status, error)")
	test.NotContains(definition, "INSERT INTO service_status (id")
}

func (test *ServiceTest) TestMigrateServiceStatusAssignsIDsWithoutNewRow() {
	ctx := context.Background()
	_, err := test.conn.Exec(ctx, `DROP TABLE service_status`)
	test.Require().NoError(err)
	_, err = test.conn.Exec(ctx, `
		CREATE TABLE service_status (
			image text REFERENCES service(image) ON DELETE CASCADE,
			status int NOT NULL,
			error text,
			occurred_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`)
	test.Require().NoError(err)

	_, err = test.conn.Exec(ctx, `INSERT INTO service (image) VALUES ('old-image')`)
	test.Require().NoError(err)

	var before int
	err = test.conn.QueryRow(ctx, `SELECT COUNT(*) FROM service_status`).Scan(&before)
	test.Require().NoError(err)
	test.Equal(1, before)

	test.Require().NoError(repository.MigrateServiceStatus(ctx, test.conn))
	test.Require().NoError(repository.MigrateServiceStatus(ctx, test.conn))
	requireIdentityPrimaryKey(&test.Suite, test.conn, "service_status")

	var after int
	var id int64
	err = test.conn.QueryRow(ctx, `SELECT COUNT(*), MIN(id) FROM service_status`).Scan(&after, &id)
	test.Require().NoError(err)
	test.Equal(1, after)
	test.NotZero(id)

	_, err = test.conn.Exec(ctx, `INSERT INTO service (image) VALUES ('new-image')`)
	test.Require().NoError(err)
	err = test.conn.QueryRow(ctx, `SELECT COUNT(*) FROM service_status WHERE image = 'new-image'`).Scan(&after)
	test.Require().NoError(err)
	test.Equal(1, after)
}

func (test *ServiceTest) TestMigrateServiceStatusLeavesDuplicateIDs() {
	ctx := context.Background()
	_, err := test.conn.Exec(ctx, `DROP TABLE service_status`)
	test.Require().NoError(err)
	_, err = test.conn.Exec(ctx, `
		CREATE TABLE service_status (
			image text,
			status int NOT NULL,
			id bigint
		)`)
	test.Require().NoError(err)
	_, err = test.conn.Exec(ctx, `
		INSERT INTO service_status (image, status, id) VALUES
			('a', 0, 1),
			('b', 0, 1)`)
	test.Require().NoError(err)

	err = repository.MigrateServiceStatus(ctx, test.conn)
	test.Require().ErrorContains(err, "service_status")

	var n int
	err = test.conn.QueryRow(ctx, `SELECT COUNT(*) FROM service_status WHERE id = 1`).Scan(&n)
	test.Require().NoError(err)
	test.Equal(2, n)
}

func requireIdentityPrimaryKey(t *suite.Suite, conn *pgxpool.Pool, table string) {
	t.T().Helper()

	var dataType, generation string
	var primaryKey bool
	err := conn.QueryRow(context.Background(), `
		SELECT c.data_type, c.identity_generation,
			EXISTS (
				SELECT 1
				FROM pg_constraint k
				JOIN pg_class rel ON rel.oid = k.conrelid
				JOIN pg_namespace n ON n.oid = rel.relnamespace
				JOIN pg_attribute a ON a.attrelid = rel.oid AND a.attnum = ANY (k.conkey)
				WHERE n.nspname = current_schema()
				  AND rel.relname = $1
				  AND k.contype = 'p'
				  AND a.attname = 'id'
				  AND array_length(k.conkey, 1) = 1
			)
		FROM information_schema.columns c
		WHERE c.table_schema = current_schema()
		  AND c.table_name = $1
		  AND c.column_name = 'id'`, table).Scan(&dataType, &generation, &primaryKey)
	t.Require().NoError(err)
	t.Equal("bigint", dataType)
	t.Equal("BY DEFAULT", generation)
	t.True(primaryKey)
}
