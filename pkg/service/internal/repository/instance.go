package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/lhjnilsson/foreverbull/internal/postgres"
	internal_pb "github.com/lhjnilsson/foreverbull/pkg/pb"
	pb "github.com/lhjnilsson/foreverbull/pkg/pb/service"
)

const InstanceTable = `CREATE TABLE IF NOT EXISTS service_instance (
id text PRIMARY KEY,
image text,

host text,
port integer,

status int NOT NULL DEFAULT 0,
error TEXT);

CREATE TABLE IF NOT EXISTS service_instance_status (
	id text REFERENCES service_instance(id) ON DELETE CASCADE,
	status int NOT NULL,
	error text,
	occurred_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE OR REPLACE FUNCTION notify_service_instance_status() RETURNS TRIGGER AS $$
BEGIN
	-- Only update service_instance_status if the status column is updated
	IF (TG_OP = 'UPDATE' AND OLD.status <> NEW.status) OR TG_OP = 'INSERT' THEN
		INSERT INTO service_instance_status (id, status, error)
		VALUES (NEW.id, NEW.status, NEW.error);
		PERFORM pg_notify('service_instance_status', NEW.id);
	END IF;
	RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DO
$$BEGIN
	CREATE TRIGGER service_instance_status_trigger AFTER INSERT OR UPDATE ON service_instance
	FOR EACH ROW EXECUTE PROCEDURE notify_service_instance_status();
EXCEPTION
	WHEN duplicate_object THEN
		NULL;
END$$;
`

type Instance struct {
	Conn postgres.Query
}

func (db *Instance) Create(ctx context.Context, instanceID string, image *string) (*pb.Instance, error) {
	_, err := db.Conn.Exec(ctx,
		`INSERT INTO service_instance (id, image) VALUES ($1, $2)`,
		instanceID, image,
	)
	if err != nil {
		return nil, fmt.Errorf("error creating instance: %w", err)
	}

	return db.Get(ctx, instanceID)
}

func (db *Instance) Get(ctx context.Context, instanceID string) (*pb.Instance, error) {
	instance := pb.Instance{}

	rows, err := db.Conn.Query(ctx,
		`SELECT service_instance.id, image, host, port, sis.status, sis.error, sis.occurred_at
		FROM service_instance
		INNER JOIN (
			SELECT id, status, error, occurred_at FROM service_instance_status ORDER BY occurred_at ASC
		) AS sis ON service_instance.id = sis.id
		WHERE service_instance.id=$1`, instanceID)
	if err != nil {
		return nil, fmt.Errorf("error getting instance: %w", err)
	}

	defer rows.Close()

	for rows.Next() {
		status := pb.Instance_Status{}
		occurredAt := time.Time{}

		err = rows.Scan(&instance.ID, &instance.Image, &instance.Host, &instance.Port,
			&status.Status, &status.Error, &occurredAt)
		if err != nil {
			return nil, fmt.Errorf("error scanning instance: %w", err)
		}

		status.OccurredAt = internal_pb.TimeToProtoTimestamp(occurredAt)
		instance.Statuses = append(instance.Statuses, &status)
	}

	if instance.ID == "" {
		return nil, errors.New("instance not found")
	}

	return &instance, nil
}

func (db *Instance) UpdateHostPort(ctx context.Context, instanceID, host string, port int) error {
	_, err := db.Conn.Exec(ctx,
		`UPDATE service_instance SET host=$1, port=$2 WHERE id=$3`,
		host, port, instanceID,
	)
	if err != nil {
		return fmt.Errorf("error updating host and port: %w", err)
	}

	return nil
}

func (db *Instance) UpdateStatus(ctx context.Context, instanceID string, status pb.Instance_Status_Status, err error) error {
	if err != nil {
		_, err = db.Conn.Exec(ctx,
			`UPDATE service_instance SET status=$2, error=$3 WHERE id=$1`,
			instanceID, status, err.Error(),
		)
	} else {
		_, err = db.Conn.Exec(ctx,
			`UPDATE service_instance SET status=$2 WHERE id=$1`,
			instanceID, status,
		)
	}

	if err != nil {
		return fmt.Errorf("error updating status: %w", err)
	}

	return nil
}

func (db *Instance) parseRows(rows pgx.Rows) ([]*pb.Instance, error) {
	instances := make([]*pb.Instance, 0)

	var inReturnSlice bool

	for rows.Next() {
		status := pb.Instance_Status{}
		occurredAt := time.Time{}
		instance := pb.Instance{}

		err := rows.Scan(
			&instance.ID, &instance.Image, &instance.Host, &instance.Port,
			&status.Status, &status.Error, &occurredAt)
		if err != nil {
			return nil, fmt.Errorf("error scanning instance: %w", err)
		}

		status.OccurredAt = internal_pb.TimeToProtoTimestamp(occurredAt)
		inReturnSlice = false

		for index := range instances {
			if instances[index].ID == instance.ID {
				instances[index].Statuses = append(instances[index].Statuses, &status)
				inReturnSlice = true
			}
		}

		if !inReturnSlice {
			instance.Statuses = append(instance.Statuses, &status)
			instances = append(instances, &instance)
		}
	}

	return instances, nil
}

func (db *Instance) List(ctx context.Context) ([]*pb.Instance, error) {
	rows, err := db.Conn.Query(ctx,
		`SELECT service_instance.id, image, host, port, sis.status, sis.error, sis.occurred_at
		FROM service_instance
		INNER JOIN (
			SELECT id, status, error, occurred_at FROM service_instance_status ORDER BY occurred_at ASC
		) AS sis ON service_instance.id = sis.id
		ORDER BY sis.occurred_at ASC`)
	if err != nil {
		return nil, fmt.Errorf("error listing instances: %w", err)
	}

	defer rows.Close()

	return db.parseRows(rows)
}

func (db *Instance) ListByImage(ctx context.Context, image string) ([]*pb.Instance, error) {
	rows, err := db.Conn.Query(ctx,
		`SELECT service_instance.id, image, host, port, sis.status, sis.error, sis.occurred_at
		FROM service_instance
		INNER JOIN (
			SELECT id, status, error, occurred_at FROM service_instance_status ORDER BY occurred_at ASC
		) AS sis ON service_instance.id = sis.id
		WHERE image=$1
		ORDER BY sis.occurred_at ASC`, image)
	if err != nil {
		return nil, fmt.Errorf("error listing instances by image: %w", err)
	}

	defer rows.Close()

	return db.parseRows(rows)
}
