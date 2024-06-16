package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/lhjnilsson/foreverbull/internal/postgres"
	"github.com/lhjnilsson/foreverbull/pkg/service/entity"
)

const ServiceTable = `CREATE TABLE IF NOT EXISTS service (
image text PRIMARY KEY,
algorithm JSONB,
status TEXT NOT NULL DEFAULT 'CREATED',
error TEXT);

CREATE TABLE IF NOT EXISTS service_status (
	image text REFERENCES service(image) ON DELETE CASCADE,
	status text NOT NULL,
	error text,
	occurred_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE OR REPLACE FUNCTION notify_service_status() RETURNS TRIGGER AS $$
BEGIN
	-- Only update service_status if the status column is updated
	IF (TG_OP = 'UPDATE' AND OLD.status <> NEW.status) OR TG_OP = 'INSERT' THEN
		INSERT INTO service_status (image, status, error)
		VALUES (NEW.image, NEW.status, NEW.error);
		PERFORM pg_notify('service_status', NEW.image);
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

func (db *Service) Create(ctx context.Context, image string) (*entity.Service, error) {
	_, err := db.Conn.Exec(ctx,
		`INSERT INTO service (image) VALUES ($1)`,
		image,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create service: %w", err)
	}
	return db.Get(ctx, image)
}

func (db *Service) Get(ctx context.Context, image string) (*entity.Service, error) {
	s := entity.Service{}
	rows, err := db.Conn.Query(ctx,
		`SELECT service.image, algorithm, ss.status, ss.error, ss.occurred_at
		FROM service
		INNER JOIN (
			SELECT image, status, error, occurred_at FROM service_status ORDER BY occurred_at DESC
		) ss ON service.image = ss.image 
		WHERE service.image=$1`, image)
	if err != nil {
		return nil, fmt.Errorf("failed to get service: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		ss := entity.ServiceStatus{}
		err = rows.Scan(
			&s.Image, &s.Algorithm, &ss.Status, &ss.Error, &ss.OccurredAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to get service: %w", err)
		}
		s.Statuses = append(s.Statuses, ss)
	}
	if s.Image == "" {
		return nil, &pgconn.PgError{Code: "02000"}
	}
	return &s, nil
}

func (db *Service) SetAlgorithm(ctx context.Context, image string, algorithm *entity.Algorithm) error {
	_, err := db.Conn.Exec(ctx,
		`UPDATE service SET algorithm=$2 WHERE image=$1`,
		image, algorithm,
	)
	return err
}

func (db *Service) UpdateStatus(ctx context.Context, image string, status entity.ServiceStatusType, err error) error {
	if err != nil {
		_, err = db.Conn.Exec(ctx,
			`UPDATE service SET status=$2, error=$3 WHERE image=$1`,
			image, status, err.Error(),
		)
	} else {
		_, err = db.Conn.Exec(ctx,
			`UPDATE service SET status=$2 WHERE image=$1`,
			image, status,
		)
	}
	return err
}

func (db *Service) List(ctx context.Context) (*[]entity.Service, error) {
	rows, err := db.Conn.Query(ctx,
		`SELECT service.image, algorithm, ss.status, ss.error, ss.occurred_at
		FROM service
		INNER JOIN (
			SELECT image, status, error, occurred_at FROM service_status ORDER BY occurred_at DESC
		) ss ON service.image = ss.image 
		ORDER BY ss.occurred_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	services := []entity.Service{}
	var inReturnSlice bool
	for rows.Next() {
		ss := entity.ServiceStatus{}
		s := entity.Service{}
		err = rows.Scan(
			&s.Image, &s.Algorithm, &ss.Status, &ss.Error, &ss.OccurredAt,
		)
		if err != nil {
			return nil, err
		}
		inReturnSlice = false
		for i := range services {
			if services[i].Image == s.Image {
				services[i].Statuses = append(services[i].Statuses, ss)
				inReturnSlice = true
			}
		}
		if !inReturnSlice {
			s.Statuses = append(s.Statuses, ss)
			services = append(services, s)
		}
	}
	return &services, nil
}

func (db *Service) Delete(ctx context.Context, image string) error {
	_, err := db.Conn.Exec(ctx,
		"DELETE FROM service WHERE image=$1", image)
	return err
}
