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

	if _, err := conn.Exec(ctx, ServiceTable); err != nil {
		return fmt.Errorf("error creating service table: %w", err)
	}

	if _, err := conn.Exec(ctx, InstanceTable); err != nil {
		return fmt.Errorf("error creating instance table: %w", err)
	}

	return nil
}

func CreateTables(ctx context.Context, conn *pgxpool.Pool) error {
	if _, err := conn.Exec(ctx, ServiceTable); err != nil {
		return fmt.Errorf("error creating service table: %w", err)
	}

	if _, err := conn.Exec(ctx, InstanceTable); err != nil {
		return fmt.Errorf("error creating instance table: %w", err)
	}

	return nil
}
