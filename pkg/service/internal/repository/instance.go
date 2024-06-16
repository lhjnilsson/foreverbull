package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/lhjnilsson/foreverbull/internal/postgres"
	"github.com/lhjnilsson/foreverbull/pkg/service/entity"
)

const InstanceTable = `CREATE TABLE IF NOT EXISTS service_instance (
id text PRIMARY KEY,
image text,

host text,
port integer,
broker_port integer,
namespace_port integer,
database_url text,
functions JSONB,

status text NOT NULL DEFAULT 'CREATED',
error TEXT);

CREATE TABLE IF NOT EXISTS service_instance_status (
	id text REFERENCES service_instance(id) ON DELETE CASCADE,
	status text NOT NULL,
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

func (db *Instance) Create(ctx context.Context, id string, image *string) (*entity.Instance, error) {
	_, err := db.Conn.Exec(ctx,
		`INSERT INTO service_instance (id, image) VALUES ($1, $2)`,
		id, image,
	)
	if err != nil {
		return nil, fmt.Errorf("error creating instance: %w", err)
	}
	return db.Get(ctx, id)
}

func (db *Instance) Get(ctx context.Context, id string) (*entity.Instance, error) {
	i := entity.Instance{}
	rows, err := db.Conn.Query(ctx,
		`SELECT service_instance.id, image, host, port, broker_port, namespace_port, database_url, functions,
		sis.status, sis.error, sis.occurred_at
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
		status := entity.InstanceStatus{}
		err = rows.Scan(&i.ID, &i.Image, &i.Host, &i.Port, &i.BrokerPort, &i.NamespacePort, &i.DatabaseURL, &i.Functions,
			&status.Status, &status.Error, &status.OccurredAt)
		if err != nil {
			return nil, fmt.Errorf("error scanning instance: %w", err)
		}
		i.Statuses = append(i.Statuses, status)
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

func (db *Instance) UpdateBrokerPort(ctx context.Context, id string, brokerPort int) error {
	_, err := db.Conn.Exec(ctx,
		`UPDATE service_instance SET broker_port=$2 WHERE id=$1`,
		id, brokerPort,
	)
	return err
}

func (db *Instance) UpdateNamespacePort(ctx context.Context, id string, namespacePort int) error {
	_, err := db.Conn.Exec(ctx,
		`UPDATE service_instance SET namespace_port=$2 WHERE id=$1`,
		id, namespacePort,
	)
	return err
}

func (db *Instance) UpdateDatabaseURL(ctx context.Context, id, databaseURL string) error {
	_, err := db.Conn.Exec(ctx,
		`UPDATE service_instance SET database_url=$2 WHERE id=$1`,
		id, databaseURL,
	)
	return err
}

func (db *Instance) UpdateFunctions(ctx context.Context, id string, functions *map[string]entity.InstanceFunction) error {
	_, err := db.Conn.Exec(ctx,
		`UPDATE service_instance SET functions=$2 WHERE id=$1`,
		id, functions,
	)
	return err
}

func (db *Instance) UpdateStatus(ctx context.Context, id string, status entity.InstanceStatusType, err error) error {
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

func (db *Instance) List(ctx context.Context) (*[]entity.Instance, error) {
	rows, err := db.Conn.Query(ctx,
		`SELECT service_instance.id, image, host, port, broker_port, namespace_port, database_url, functions,
		sis.status, sis.error, sis.occurred_at
		FROM service_instance
		INNER JOIN (
			SELECT id, status, error, occurred_at FROM service_instance_status ORDER BY occurred_at DESC
		) AS sis ON service_instance.id = sis.id
		ORDER BY sis.occurred_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	instances := make([]entity.Instance, 0)
	var inReturnSlice bool
	for rows.Next() {
		status := entity.InstanceStatus{}
		i := entity.Instance{}
		err = rows.Scan(&i.ID, &i.Image, &i.Host, &i.Port, &i.BrokerPort, &i.NamespacePort, &i.DatabaseURL, &i.Functions,
			&status.Status, &status.Error, &status.OccurredAt)
		if err != nil {
			return nil, err
		}
		inReturnSlice = false
		for index := range instances {
			if instances[index].ID == i.ID {
				instances[index].Statuses = append(instances[index].Statuses, status)
				inReturnSlice = true
			}
		}
		if !inReturnSlice {
			i.Statuses = append(i.Statuses, status)
			instances = append(instances, i)
		}
	}
	return &instances, nil
}

func (db *Instance) ListByImage(ctx context.Context, image string) (*[]entity.Instance, error) {
	rows, err := db.Conn.Query(ctx,
		`SELECT service_instance.id, image, host, port, broker_port, namespace_port, database_url, functions,
		sis.status, sis.error, sis.occurred_at
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

	instances := make([]entity.Instance, 0)
	var inReturnSlice bool
	for rows.Next() {
		status := entity.InstanceStatus{}
		i := entity.Instance{}
		err = rows.Scan(&i.ID, &i.Image, &i.Host, &i.Port, &i.BrokerPort, &i.NamespacePort, &i.DatabaseURL, &i.Functions,
			&status.Status, &status.Error, &status.OccurredAt)
		if err != nil {
			return nil, err
		}
		inReturnSlice = false
		for index := range instances {
			if instances[index].ID == i.ID {
				instances[index].Statuses = append(instances[index].Statuses, status)
				inReturnSlice = true
			}
		}
		if !inReturnSlice {
			i.Statuses = append(i.Statuses, status)
			instances = append(instances, i)
		}
	}
	return &instances, nil
}
