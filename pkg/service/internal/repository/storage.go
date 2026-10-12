package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

func Recreate(ctx context.Context, conn *pgxpool.Pool) error {
	if _, err := conn.Exec(ctx, `DROP TABLE IF EXISTS service_instance_status;`); err != nil {
		return fmt.Errorf("error dropping service_instance_status: %w", err)
	}

	if _, err := conn.Exec(ctx, `DROP TABLE IF EXISTS service_instance;`); err != nil {
		return fmt.Errorf("error dropping service_instance: %w", err)
	}

	if _, err := conn.Exec(ctx, `DROP TABLE IF EXISTS service_status;`); err != nil {
		return fmt.Errorf("error dropping service_status: %w", err)
	}

	if _, err := conn.Exec(ctx, `DROP TABLE IF EXISTS service;`); err != nil {
		return fmt.Errorf("error dropping service: %w", err)
	}

	return createServiceTables(ctx, conn)
}

func CreateTables(ctx context.Context, conn *pgxpool.Pool) error {
	return createServiceTables(ctx, conn)
}

func createServiceTables(ctx context.Context, conn *pgxpool.Pool) error {
	if _, err := conn.Exec(ctx, ServiceTable); err != nil {
		return fmt.Errorf("error creating service table: %w", err)
	}

	if err := MigrateServiceStatus(ctx, conn); err != nil {
		return err
	}

	if _, err := conn.Exec(ctx, InstanceTable); err != nil {
		return fmt.Errorf("error creating instance table: %w", err)
	}

	return MigrateInstanceStatus(ctx, conn)
}
