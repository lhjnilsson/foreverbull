package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	internal_pb "github.com/lhjnilsson/foreverbull/internal/pb"
	"github.com/lhjnilsson/foreverbull/internal/postgres"
	"github.com/lhjnilsson/foreverbull/pkg/service/pb"
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

func (db *Instance) Create(ctx context.Context, id string, image *string) (*pb.Instance, error) {
	_, err := db.Conn.Exec(ctx,
		`INSERT INTO service_instance (id, image) VALUES ($1, $2)`,
		id, image,
	)
	if err != nil {
		return nil, fmt.Errorf("error creating instance: %w", err)
	}
	return db.Get(ctx, id)
}

func (db *Instance) Get(ctx context.Context, id string) (*pb.Instance, error) {
	i := pb.Instance{}
	rows, err := db.Conn.Query(ctx,
		`SELECT service_instance.id, image, host, port, sis.status, sis.error, sis.occurred_at
		FROM service_instance
		INNER JOIN (
			SELECT id, status, error, occurred_at FROM service_instance_status ORDER BY occurred_at DESC
		) AS sis ON service_instance.id = sis.id
		WHERE service_instance.id=$1`, id)
	if err != nil {
		return nil, fmt.Errorf("error getting instance: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		status := pb.Instance_Status{}
		t := time.Time{}
		err = rows.Scan(&i.ID, &i.Image, &i.Host, &i.Port, &status.Status, &status.Error, &t)
		if err != nil {
			return nil, fmt.Errorf("error scanning instance: %w", err)
		}
		status.OccurredAt = internal_pb.TimeToProtoTimestamp(t)
		i.Statuses = append(i.Statuses, &status)
	}
	if i.ID == "" {
		return nil, &pgconn.PgError{Code: "02000"}
	}
	return &i, nil
}

func (db *Instance) UpdateHostPort(ctx context.Context, id, host string, port int) error {
	_, err := db.Conn.Exec(ctx,
		`UPDATE service_instance SET host=$1, port=$2 WHERE id=$3`,
		host, port, id,
	)
	if err != nil {
		return err
	}
	return nil
}

func (db *Instance) UpdateStatus(ctx context.Context, id string, status pb.Instance_Status_Status, err error) error {
	if err != nil {
		_, err = db.Conn.Exec(ctx,
			`UPDATE service_instance SET status=$2, error=$3 WHERE id=$1`,
			id, status, err.Error(),
		)
	} else {
		_, err = db.Conn.Exec(ctx,
			`UPDATE service_instance SET status=$2 WHERE id=$1`,
			id, status,
		)
	}
	return err
}

func (db *Instance) parseRows(rows pgx.Rows) ([]*pb.Instance, error) {
	instances := make([]*pb.Instance, 0)
	var inReturnSlice bool
	for rows.Next() {
		status := pb.Instance_Status{}
		t := time.Time{}
		i := pb.Instance{}
		err := rows.Scan(
			&i.ID, &i.Image, &i.Host, &i.Port,
			&status.Status, &status.Error, &t)
		if err != nil {
			return nil, err
		}
		status.OccurredAt = internal_pb.TimeToProtoTimestamp(t)
		inReturnSlice = false
		for index := range instances {
			if instances[index].ID == i.ID {
				instances[index].Statuses = append(instances[index].Statuses, &status)
				inReturnSlice = true
			}
		}
		if !inReturnSlice {
			i.Statuses = append(i.Statuses, &status)
			instances = append(instances, &i)
		}
	}
	return instances, nil
}

func (db *Instance) List(ctx context.Context) ([]*pb.Instance, error) {
	rows, err := db.Conn.Query(ctx,
		`SELECT service_instance.id, image, host, port, sis.status, sis.error, sis.occurred_at
		FROM service_instance
		INNER JOIN (
			SELECT id, status, error, occurred_at FROM service_instance_status ORDER BY occurred_at DESC
		) AS sis ON service_instance.id = sis.id
		ORDER BY sis.occurred_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return db.parseRows(rows)
}

func (db *Instance) ListByImage(ctx context.Context, image string) ([]*pb.Instance, error) {
	rows, err := db.Conn.Query(ctx,
		`SELECT service_instance.id, image, host, port, sis.status, sis.error, sis.occurred_at
		FROM service_instance
		INNER JOIN (
			SELECT id, status, error, occurred_at FROM service_instance_status ORDER BY occurred_at DESC
		) AS sis ON service_instance.id = sis.id
		WHERE image=$1
		ORDER BY sis.occurred_at DESC`, image)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return db.parseRows(rows)
}
