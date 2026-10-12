package repository_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/internal/test_helper"
	pb "github.com/lhjnilsson/foreverbull/pkg/pb/service"
	"github.com/lhjnilsson/foreverbull/pkg/service/internal/repository"
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

func (test *InstanceTest) TestInstanceStatusPrimaryKey() {
	ctx := context.Background()
	test.Require().NoError(repository.MigrateInstanceStatus(ctx, test.conn))
	test.Require().NoError(repository.MigrateInstanceStatus(ctx, test.conn))
	requireIdentityPrimaryKey(&test.Suite, test.conn, "service_instance_status")

	instances := &repository.Instance{Conn: test.conn}
	_, err := instances.Create(ctx, "instance", nil)
	test.Require().NoError(err)

	var n int
	err = test.conn.QueryRow(ctx, `SELECT COUNT(*) FROM service_instance_status WHERE instance = 'instance'`).Scan(&n)
	test.Require().NoError(err)
	test.Equal(1, n)

	test.Require().NoError(repository.MigrateInstanceStatus(ctx, test.conn))
	err = test.conn.QueryRow(ctx, `SELECT COUNT(*) FROM service_instance_status WHERE instance = 'instance'`).Scan(&n)
	test.Require().NoError(err)
	test.Equal(1, n)
}

func (test *InstanceTest) TestMigrateInstanceStatusAssignsIDsWithoutNewRow() {
	ctx := context.Background()
	_, err := test.conn.Exec(ctx, `DROP TABLE service_instance_status`)
	test.Require().NoError(err)
	_, err = test.conn.Exec(ctx, `
		CREATE TABLE service_instance_status (
			id text REFERENCES service_instance(id) ON DELETE CASCADE,
			status int NOT NULL,
			error text,
			occurred_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`)
	test.Require().NoError(err)
	_, err = test.conn.Exec(ctx, `
		CREATE OR REPLACE FUNCTION notify_service_instance_status() RETURNS TRIGGER AS $$
		BEGIN
			IF (TG_OP = 'UPDATE' AND OLD.status <> NEW.status) OR TG_OP = 'INSERT' THEN
				INSERT INTO service_instance_status (id, status, error)
				VALUES (NEW.id, NEW.status, NEW.error);
				PERFORM pg_notify('service_instance_status', NEW.id);
			END IF;
			RETURN NEW;
		END;
		$$ LANGUAGE plpgsql`)
	test.Require().NoError(err)

	_, err = test.conn.Exec(ctx, `INSERT INTO service_instance (id) VALUES ('old-instance')`)
	test.Require().NoError(err)

	var before int
	err = test.conn.QueryRow(ctx, `SELECT COUNT(*) FROM service_instance_status`).Scan(&before)
	test.Require().NoError(err)
	test.Equal(1, before)

	test.Require().NoError(repository.MigrateInstanceStatus(ctx, test.conn))
	test.Require().NoError(repository.MigrateInstanceStatus(ctx, test.conn))
	requireIdentityPrimaryKey(&test.Suite, test.conn, "service_instance_status")

	var after int
	var id int64
	var instance string
	err = test.conn.QueryRow(ctx, `
		SELECT COUNT(*), MIN(id), MIN(instance) FROM service_instance_status`).Scan(&after, &id, &instance)
	test.Require().NoError(err)
	test.Equal(before, after)
	test.NotZero(id)
	test.Equal("old-instance", instance)

	var definition string
	err = test.conn.QueryRow(ctx, `SELECT pg_get_functiondef('notify_service_instance_status()'::regprocedure)`).Scan(&definition)
	test.Require().NoError(err)
	test.Contains(definition, "INSERT INTO service_instance_status (instance, status, error)")
	test.NotContains(definition, "INSERT INTO service_instance_status (id")

	instances := &repository.Instance{Conn: test.conn}
	_, err = instances.Create(ctx, "new-instance", nil)
	test.Require().NoError(err)
	err = test.conn.QueryRow(ctx, `SELECT COUNT(*) FROM service_instance_status WHERE instance = 'new-instance'`).Scan(&after)
	test.Require().NoError(err)
	test.Equal(1, after)
}
