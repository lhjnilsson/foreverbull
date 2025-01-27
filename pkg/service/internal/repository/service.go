package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/lhjnilsson/foreverbull/internal/postgres"
	internal_pb "github.com/lhjnilsson/foreverbull/pkg/pb"
	"github.com/lhjnilsson/foreverbull/pkg/service/pb"
)

const ServiceTable = `CREATE TABLE IF NOT EXISTS service (
image text PRIMARY KEY,
algorithm JSONB,
status int NOT NULL DEFAULT 0,
error TEXT);

CREATE TABLE IF NOT EXISTS service_status (
	image text REFERENCES service(image) ON DELETE CASCADE,
	status int NOT NULL,
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

func (db *Service) Create(ctx context.Context, image string) (*pb.Service, error) {
	_, err := db.Conn.Exec(ctx,
		`INSERT INTO service (image) VALUES ($1)`,
		image,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create service: %w", err)
	}

	return db.Get(ctx, image)
}

func (db *Service) Get(ctx context.Context, image string) (*pb.Service, error) {
	service := pb.Service{}

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

	algorithm := []byte{}

	for rows.Next() {
		status := pb.Service_Status{}
		t := time.Time{}
		err = rows.Scan(
			&service.Image, &algorithm, &status.Status, &status.Error, &t,
		)
		status.OccurredAt = internal_pb.TimeToProtoTimestamp(t)

		if err != nil {
			return nil, fmt.Errorf("failed to get service: %w", err)
		}

		service.Statuses = append(service.Statuses, &status)
	}

	if service.Image == "" {
		return nil, errors.New("service not found")
	}

	if algorithm == nil {
		return &service, nil
	}

	err = json.Unmarshal(algorithm, &service.Algorithm)
	if err != nil {
		return nil, fmt.Errorf("failed to decode algorithm: %w", err)
	}

	return &service, nil
}

func (db *Service) SetAlgorithm(ctx context.Context, image string, a *pb.Algorithm) error {
	algorithm, err := json.Marshal(a)
	if err != nil {
		return fmt.Errorf("failed to encode algorithm: %w", err)
	}

	_, err = db.Conn.Exec(ctx,
		`UPDATE service SET algorithm=$2 WHERE image=$1`,
		image, algorithm,
	)
	if err != nil {
		return fmt.Errorf("failed to set algorithm: %w", err)
	}

	return nil
}

func (db *Service) UpdateStatus(ctx context.Context, image string, status pb.Service_Status_Status, err error) error {
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

	if err != nil {
		return fmt.Errorf("failed to update status: %w", err)
	}

	return nil
}

func (db *Service) List(ctx context.Context) ([]*pb.Service, error) {
	rows, err := db.Conn.Query(ctx,
		`SELECT service.image, algorithm, ss.status, ss.error, ss.occurred_at
		FROM service
		INNER JOIN (
			SELECT image, status, error, occurred_at FROM service_status ORDER BY occurred_at DESC
		) ss ON service.image = ss.image
		ORDER BY ss.occurred_at ASC`)
	if err != nil {
		return nil, fmt.Errorf("failed to list services: %w", err)
	}
	defer rows.Close()

	services := []*pb.Service{}

	var inReturnSlice bool

	for rows.Next() {
		serviceStatus := pb.Service_Status{}
		occurredAt := time.Time{}
		service := pb.Service{}
		algorithm := []byte{}

		err = rows.Scan(
			&service.Image, &algorithm, &serviceStatus.Status, &serviceStatus.Error, &occurredAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to list services: %w", err)
		}

		serviceStatus.OccurredAt = internal_pb.TimeToProtoTimestamp(occurredAt)

		inReturnSlice = false

		for i := range services {
			if services[i].Image == service.Image {
				services[i].Statuses = append(services[i].Statuses, &serviceStatus)
				inReturnSlice = true
			}
		}

		if !inReturnSlice {
			service.Statuses = append(service.Statuses, &serviceStatus)
			services = append(services, &service)
		}

		if algorithm == nil {
			continue
		}

		err = json.Unmarshal(algorithm, &service.Algorithm)
		if err != nil {
			return nil, fmt.Errorf("failed to decode algorithm: %w", err)
		}
	}

	return services, nil
}

func (db *Service) Delete(ctx context.Context, image string) error {
	_, err := db.Conn.Exec(ctx,
		"DELETE FROM service WHERE image=$1", image)
	if err != nil {
		return fmt.Errorf("failed to delete service: %w", err)
	}

	return nil
}
