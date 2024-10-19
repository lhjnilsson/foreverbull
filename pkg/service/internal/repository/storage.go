package repository

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

func Recreate(ctx context.Context, conn *pgxpool.Pool) error {
	if _, err := conn.Exec(ctx, `DROP TABLE IF EXISTS service_instance_status;`); err != nil {
		return err
	}

	if _, err := conn.Exec(ctx, `DROP TABLE IF EXISTS service_instance;`); err != nil {
		return err
	}

	if _, err := conn.Exec(ctx, `DROP TABLE IF EXISTS service_status;`); err != nil {
		return err
	}

	if _, err := conn.Exec(ctx, `DROP TABLE IF EXISTS service;`); err != nil {
		return err
	}

	if _, err := conn.Exec(ctx, ServiceTable); err != nil {
		return err
	}

	if _, err := conn.Exec(ctx, InstanceTable); err != nil {
		return err
	}

	return nil
}

func CreateTables(ctx context.Context, conn *pgxpool.Pool) error {
	if _, err := conn.Exec(ctx, ServiceTable); err != nil {
		return err
	}

	if _, err := conn.Exec(ctx, InstanceTable); err != nil {
		return err
	}

	return nil
}
