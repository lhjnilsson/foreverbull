package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/lhjnilsson/foreverbull/internal/auth"
	"github.com/lhjnilsson/foreverbull/internal/postgres"
	"github.com/lhjnilsson/foreverbull/pkg/service"
)

// MigrateStrategyOwner assigns strategy.owner_id and replaces the global name
// unique with (owner_id, name). strategy.id stays the primary key. Service
// images are backfilled onto strategy_version.service_id while service.image
// is still unique. It does not search for an editor and it does not insert a
// password. Calling it twice is safe. Startup calls it when authentication is on.
func MigrateStrategyOwner(ctx context.Context, conn *pgxpool.Pool, adminID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if conn == nil {
		return fmt.Errorf("strategy: nil database")
	}
	if err := auth.Migrate(ctx, conn); err != nil {
		return err
	}
	if err := auth.RequireAccount(ctx, conn, "strategy", adminID); err != nil {
		return err
	}
	// Service owns the uuid primary key. Call it even when that key is already
	// in place so a second call can finish strategy_version.service_id.
	if err := service.MigrateOwner(ctx, conn, adminID); err != nil {
		return err
	}
	exists, err := auth.TableExists(ctx, conn, "strategy")
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("strategy: table is missing")
	}
	done, err := strategyOwnerComplete(ctx, conn)
	if err != nil {
		return err
	}
	if done {
		return auth.Migrate(ctx, conn)
	}
	if err = auth.WithTx(ctx, conn, "strategy", func(tx pgx.Tx) error {
		return migrateStrategyOwnerTx(ctx, tx, adminID)
	}); err != nil {
		return err
	}
	return auth.Migrate(ctx, conn)
}

func strategyOwnerComplete(ctx context.Context, conn postgres.Query) (bool, error) {
	primaryKey, err := auth.PrimaryKeyColumns(ctx, conn, "strategy")
	if err != nil {
		return false, err
	}
	if !auth.SameColumns(primaryKey, []string{"id"}) {
		return false, nil
	}
	ownerNotNull, err := auth.ColumnNotNull(ctx, conn, "strategy", "owner_id")
	if err != nil || !ownerNotNull {
		return false, err
	}
	unique, err := auth.ConstraintExists(ctx, conn, "strategy", "strategy_owner_id_name_key")
	if err != nil || !unique {
		return false, err
	}
	names, err := singleColumnUniques(ctx, conn, "strategy", "name")
	if err != nil {
		return false, err
	}
	if len(names) > 0 {
		return false, nil
	}
	return true, nil
}

func migrateStrategyOwnerTx(ctx context.Context, conn postgres.Query, adminID string) error {
	if err := auth.EnsureOwner(ctx, conn, "strategy", adminID); err != nil {
		return err
	}
	if err := dropNameUnique(ctx, conn); err != nil {
		return err
	}
	return auth.EnsureUnique(ctx, conn, "strategy", "strategy_owner_id_name_key", "owner_id", "name")
}

func dropNameUnique(ctx context.Context, conn postgres.Query) error {
	names, err := singleColumnUniques(ctx, conn, "strategy", "name")
	if err != nil {
		return err
	}
	for _, name := range names {
		if err = auth.DropConstraint(ctx, conn, "strategy", name); err != nil {
			return err
		}
	}
	return nil
}

// singleColumnUniques reads the constraint name from pg_constraint. The global
// unique is not assumed to be strategy_name_key.
func singleColumnUniques(ctx context.Context, conn postgres.Query, table, column string) ([]string, error) {
	rows, err := conn.Query(ctx, `
		SELECT c.conname
		FROM pg_constraint c
		JOIN pg_class t ON t.oid = c.conrelid
		JOIN pg_namespace n ON n.oid = t.relnamespace
		JOIN pg_attribute a ON a.attrelid = t.oid AND a.attnum = ANY (c.conkey)
		WHERE n.nspname = current_schema()
		  AND t.relname = $1
		  AND c.contype = 'u'
		  AND array_length(c.conkey, 1) = 1
		  AND a.attname = $2
		ORDER BY c.conname`, table, column)
	if err != nil {
		return nil, fmt.Errorf("%s: read unique %s: %w", table, column, err)
	}
	defer rows.Close()

	names := []string{}
	for rows.Next() {
		var name string
		if err = rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("%s: scan unique %s: %w", table, column, err)
		}
		names = append(names, name)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("%s: unique %s: %w", table, column, err)
	}
	return names, nil
}
