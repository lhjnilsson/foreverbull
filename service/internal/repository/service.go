package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/lhjnilsson/foreverbull/internal/postgres"
	"github.com/lhjnilsson/foreverbull/service/entity"
)

const ServiceTable = `CREATE TABLE IF NOT EXISTS service (
name text PRIMARY KEY CONSTRAINT servicenamechk CHECK (char_length(name) > 3),
image text NOT NULL,
status TEXT NOT NULL DEFAULT 'CREATED',
error TEXT,
service_type TEXT,
worker_parameters JSONB);

CREATE TABLE IF NOT EXISTS service_status (
	name text REFERENCES service(name) ON DELETE CASCADE,
	status text NOT NULL,
	error text,
	occurred_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE OR REPLACE FUNCTION notify_service_status() RETURNS TRIGGER AS $$
BEGIN
	-- Only update service_status if the status column is updated
	IF (TG_OP = 'UPDATE' AND OLD.status <> NEW.status) OR TG_OP = 'INSERT' THEN
		INSERT INTO service_status (name, status, error)
		VALUES (NEW.name, NEW.status, NEW.error);
		PERFORM pg_notify('service_status', NEW.name);
	END IF;
	RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DO
$$BEGIN
	CREATE TRIGGER service_status_trigger AFTER INSERT OR UPDATE ON service
	FOR EACH ROW EXECUTE PROCEDURE notify_service_status();
EXCEPTION
	WHEN duplicate_object THEN
		NULL;
END$$;
`

type Service struct {
	Conn postgres.Query
}

func (db *Service) Create(ctx context.Context, name, image string) (*entity.Service, error) {
	_, err := db.Conn.Exec(ctx,
		`INSERT INTO service (name, image) VALUES ($1, $2)`,
		name, image,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create service: %w", err)
	}
	return db.Get(ctx, name)
}

func (db *Service) Get(ctx context.Context, name string) (*entity.Service, error) {
	s := entity.Service{}
	rows, err := db.Conn.Query(ctx,
		`SELECT service.name, image, service_type, worker_parameters, 
		ss.status, ss.error, ss.occurred_at
		FROM service
		INNER JOIN (
			SELECT name, status, error, occurred_at FROM service_status ORDER BY occurred_at DESC
		) ss ON service.name = ss.name 
		WHERE service.name=$1`, name)
	if err != nil {
		return nil, fmt.Errorf("failed to get service: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		status := entity.ServiceStatus{}
		err = rows.Scan(
			&s.Name, &s.Image, &s.Type, &s.WorkerParameters,
			&status.Status, &status.Error, &status.OccurredAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to get service: %w", err)
		}
		s.Statuses = append(s.Statuses, status)
	}
	if s.Name == "" {
		return nil, &pgconn.PgError{Code: "02000"}
	}
	return &s, nil
}

func (db *Service) UpdateServiceInfo(ctx context.Context, name string, serviceType string, parameters *[]entity.Parameter) error {
	_, err := db.Conn.Exec(ctx,
		`UPDATE service SET service_type=$2, worker_parameters=$3 WHERE name=$1`,
		name, serviceType, parameters,
	)
	return err
}

func (db *Service) UpdateStatus(ctx context.Context, name string, status entity.ServiceStatusType, err error) error {
	if err != nil {
		_, err = db.Conn.Exec(ctx,
			`UPDATE service SET status=$2, error=$3 WHERE name=$1`,
			name, status, err.Error(),
		)
	} else {
		_, err = db.Conn.Exec(ctx,
			`UPDATE service SET status=$2 WHERE name=$1`,
			name, status,
		)
	}
	return err
}

func (db *Service) List(ctx context.Context) (*[]entity.Service, error) {
	rows, err := db.Conn.Query(ctx,
		`SELECT service.name, image, service_type, worker_parameters,
		ss.status, ss.error, ss.occurred_at
		FROM service
		INNER JOIN (
			SELECT name, status, error, occurred_at FROM service_status ORDER BY occurred_at DESC
		) ss ON service.name = ss.name 
		ORDER BY ss.occurred_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	services := []entity.Service{}
	var inReturnSlice bool
	for rows.Next() {
		status := entity.ServiceStatus{}
		s := entity.Service{}
		err = rows.Scan(
			&s.Name, &s.Image, &s.Type, &s.WorkerParameters,
			&status.Status, &status.Error, &status.OccurredAt,
		)
		if err != nil {
			return nil, err
		}
		inReturnSlice = false
		for i := range services {
			if services[i].Name == s.Name {
				services[i].Statuses = append(services[i].Statuses, status)
				inReturnSlice = true
			}
		}
		if !inReturnSlice {
			s.Statuses = append(s.Statuses, status)
			services = append(services, s)
		}
	}
	return &services, nil
}

func (db *Service) Delete(ctx context.Context, name string) error {
	_, err := db.Conn.Exec(ctx,
		"DELETE FROM service WHERE name=$1", name)
	return err
}
