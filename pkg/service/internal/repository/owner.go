package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/lhjnilsson/foreverbull/internal/auth"
	"github.com/lhjnilsson/foreverbull/internal/postgres"
)

// serviceStatusNotifyByID stores history by service id. CreateTables still
// installs the image notify function; this replacement runs only from
// MigrateServiceOwner, after service_id is filled.
const serviceStatusNotifyByID = `CREATE OR REPLACE FUNCTION notify_service_status() RETURNS TRIGGER AS $$
BEGIN
	IF (TG_OP = 'UPDATE' AND OLD.status <> NEW.status) OR TG_OP = 'INSERT' THEN
		INSERT INTO service_status (service_id, status, error)
		VALUES (NEW.id, NEW.status, NEW.error);
		PERFORM pg_notify('service_status', NEW.id);
	END IF;
	RETURN NEW;
END;
$$ LANGUAGE plpgsql;
`

// MigrateServiceOwner moves service onto a uuid primary key and assigns
// owner_id. It does not search for an editor and it does not insert a password.
// image stays unique per owner. Calling it twice is safe. Startup does not call it.
func MigrateServiceOwner(ctx context.Context, conn *pgxpool.Pool, adminID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if conn == nil {
		return fmt.Errorf("service: nil database")
	}
	if err := auth.Migrate(ctx, conn); err != nil {
		return err
	}
	if err := auth.RequireAccount(ctx, conn, "service", adminID); err != nil {
		return err
	}
	exists, err := auth.TableExists(ctx, conn, "service")
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("service: table is missing")
	}
	done, err := serviceOwnerComplete(ctx, conn)
	if err != nil {
		return err
	}
	if done {
		return auth.Migrate(ctx, conn)
	}

	if err = auth.WithTx(ctx, conn, "service", func(tx pgx.Tx) error {
		return migrateServiceOwnerTx(ctx, tx, adminID)
	}); err != nil {
		return err
	}
	return auth.Migrate(ctx, conn)
}

func migrateServiceOwnerTx(ctx context.Context, conn postgres.Query, adminID string) error {
	if err := auth.EnsureSurrogateID(ctx, conn, "service"); err != nil {
		return err
	}
	if err := auth.EnsureOwner(ctx, conn, "service", adminID); err != nil {
		return err
	}
	if err := migrateServiceStatusOwner(ctx, conn); err != nil {
		return err
	}
	if err := migrateServiceInstanceOwner(ctx, conn); err != nil {
		return err
	}
	if err := retargetStrategyVersion(ctx, conn); err != nil {
		return err
	}
	return swapServicePrimaryKey(ctx, conn)
}

func serviceOwnerComplete(ctx context.Context, conn postgres.Query) (bool, error) {
	primaryKey, err := auth.PrimaryKeyColumns(ctx, conn, "service")
	if err != nil {
		return false, err
	}
	if !auth.SameColumns(primaryKey, []string{"id"}) {
		return false, nil
	}
	ownerNotNull, err := auth.ColumnNotNull(ctx, conn, "service", "owner_id")
	if err != nil || !ownerNotNull {
		return false, err
	}
	unique, err := auth.ConstraintExists(ctx, conn, "service", "service_owner_id_image_key")
	if err != nil || !unique {
		return false, err
	}
	ok, err := statusOwnerComplete(ctx, conn)
	if err != nil || !ok {
		return false, err
	}
	ok, err = instanceOwnerComplete(ctx, conn)
	if err != nil || !ok {
		return false, err
	}
	return versionOwnerComplete(ctx, conn)
}

func statusOwnerComplete(ctx context.Context, conn postgres.Query) (bool, error) {
	exists, err := auth.TableExists(ctx, conn, "service_status")
	if err != nil {
		return false, err
	}
	if !exists {
		return true, nil
	}
	hasImage, err := auth.ColumnExists(ctx, conn, "service_status", "image")
	if err != nil || hasImage {
		return false, err
	}
	notNull, err := auth.ColumnNotNull(ctx, conn, "service_status", "service_id")
	if err != nil || !notNull {
		return false, err
	}
	return serviceStatusNotifyCurrent(ctx, conn)
}

// serviceStatusNotifyCurrent is false when CreateTables has put the image
// insert back. The ownership migration replaces that function again.
func serviceStatusNotifyCurrent(ctx context.Context, conn postgres.Query) (bool, error) {
	var definition string
	err := conn.QueryRow(ctx, `SELECT pg_get_functiondef('notify_service_status()'::regprocedure)`).Scan(&definition)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "42883" {
			return false, nil
		}
		return false, fmt.Errorf("service_status: read notify function: %w", err)
	}
	current := strings.Contains(definition, "INSERT INTO service_status (service_id, status, error)") &&
		strings.Contains(definition, "pg_notify('service_status', NEW.id)")
	return current, nil
}

func instanceOwnerComplete(ctx context.Context, conn postgres.Query) (bool, error) {
	exists, err := auth.TableExists(ctx, conn, "service_instance")
	if err != nil || !exists {
		return true, err
	}
	hasID, err := auth.ColumnExists(ctx, conn, "service_instance", "service_id")
	if err != nil || !hasID {
		return false, err
	}
	return true, nil
}

func versionOwnerComplete(ctx context.Context, conn postgres.Query) (bool, error) {
	exists, err := auth.TableExists(ctx, conn, "strategy_version")
	if err != nil || !exists {
		return true, err
	}
	notNull, err := auth.ColumnNotNull(ctx, conn, "strategy_version", "service_id")
	if err != nil || !notNull {
		return false, err
	}
	refs, err := auth.ForeignKeysReferencing(ctx, conn, "service", "image")
	if err != nil {
		return false, err
	}
	for _, ref := range refs {
		if ref.Table == "strategy_version" {
			return false, nil
		}
	}
	return true, nil
}

func migrateServiceStatusOwner(ctx context.Context, conn postgres.Query) error {
	exists, err := auth.TableExists(ctx, conn, "service_status")
	if err != nil || !exists {
		return err
	}
	if _, err = conn.Exec(ctx, `ALTER TABLE service_status ADD COLUMN IF NOT EXISTS service_id text`); err != nil {
		return fmt.Errorf("service_status: service_id: %w", err)
	}
	hasImage, err := auth.ColumnExists(ctx, conn, "service_status", "image")
	if err != nil {
		return err
	}
	if hasImage {
		_, err = conn.Exec(ctx, `
			UPDATE service_status AS status
			SET service_id = service.id
			FROM service
			WHERE status.service_id IS NULL
			  AND status.image = service.image`)
		if err != nil {
			return fmt.Errorf("service_status: backfill service_id: %w", err)
		}
	}
	var missing bool
	err = conn.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM service_status WHERE service_id IS NULL)`).Scan(&missing)
	if err != nil {
		return fmt.Errorf("service_status: scan service_id: %w", err)
	}
	if missing {
		return fmt.Errorf("service_status: rows have no service")
	}
	if _, err = conn.Exec(ctx, `ALTER TABLE service_status ALTER COLUMN service_id SET NOT NULL`); err != nil {
		return fmt.Errorf("service_status: service_id not null: %w", err)
	}
	if err = auth.EnsureForeignKey(ctx, conn, "service_status", "service_status_service_id_fkey",
		`FOREIGN KEY (service_id) REFERENCES service (id) ON DELETE CASCADE`); err != nil {
		return err
	}
	if _, err = conn.Exec(ctx, serviceStatusNotifyByID); err != nil {
		return fmt.Errorf("service_status: notify function: %w", err)
	}
	if !hasImage {
		return nil
	}
	if err = dropImageForeignKeys(ctx, conn, "service_status"); err != nil {
		return err
	}
	if _, err = conn.Exec(ctx, `ALTER TABLE service_status DROP COLUMN image`); err != nil {
		return fmt.Errorf("service_status: drop image: %w", err)
	}
	return nil
}

func migrateServiceInstanceOwner(ctx context.Context, conn postgres.Query) error {
	exists, err := auth.TableExists(ctx, conn, "service_instance")
	if err != nil || !exists {
		return err
	}
	if _, err = conn.Exec(ctx, `ALTER TABLE service_instance ADD COLUMN IF NOT EXISTS service_id text`); err != nil {
		return fmt.Errorf("service_instance: service_id: %w", err)
	}
	_, err = conn.Exec(ctx, `
		UPDATE service_instance AS instance
		SET service_id = service.id
		FROM service
		WHERE instance.service_id IS NULL
		  AND instance.image IS NOT NULL
		  AND instance.image = service.image`)
	if err != nil {
		return fmt.Errorf("service_instance: backfill service_id: %w", err)
	}
	return auth.EnsureForeignKey(ctx, conn, "service_instance", "service_instance_service_id_fkey",
		`FOREIGN KEY (service_id) REFERENCES service (id)`)
}

func retargetStrategyVersion(ctx context.Context, conn postgres.Query) error {
	exists, err := auth.TableExists(ctx, conn, "strategy_version")
	if err != nil || !exists {
		return err
	}
	if _, err = conn.Exec(ctx, `ALTER TABLE strategy_version ADD COLUMN IF NOT EXISTS service_id text`); err != nil {
		return fmt.Errorf("strategy_version: service_id: %w", err)
	}
	var needsBackfill bool
	err = conn.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM strategy_version WHERE service_id IS NULL)`).Scan(&needsBackfill)
	if err != nil {
		return fmt.Errorf("strategy_version: scan service_id: %w", err)
	}
	if needsBackfill {
		if err = backfillStrategyVersionService(ctx, conn); err != nil {
			return err
		}
	}
	notNull, err := auth.ColumnNotNull(ctx, conn, "strategy_version", "service_id")
	if err != nil {
		return err
	}
	if !notNull {
		if _, err = conn.Exec(ctx, `ALTER TABLE strategy_version ALTER COLUMN service_id SET NOT NULL`); err != nil {
			return fmt.Errorf("strategy_version: service_id not null: %w", err)
		}
	}
	if err = auth.EnsureForeignKey(ctx, conn, "strategy_version", "strategy_version_service_id_fkey",
		`FOREIGN KEY (service_id) REFERENCES service (id)`); err != nil {
		return err
	}
	return dropImageForeignKeys(ctx, conn, "strategy_version")
}

func backfillStrategyVersionService(ctx context.Context, conn postgres.Query) error {
	var unique bool
	err := conn.QueryRow(ctx, `
		SELECT NOT EXISTS (SELECT 1 FROM service GROUP BY image HAVING COUNT(*) > 1)
		   AND NOT EXISTS (SELECT 1 FROM service WHERE image IS NULL)`).Scan(&unique)
	if err != nil {
		return fmt.Errorf("service: scan image: %w", err)
	}
	if !unique {
		return fmt.Errorf("service: image is not unique")
	}
	_, err = conn.Exec(ctx, `
		UPDATE strategy_version AS version
		SET service_id = service.id
		FROM service
		WHERE version.service_id IS NULL
		  AND version.service_image = service.image`)
	if err != nil {
		return fmt.Errorf("strategy_version: backfill service_id: %w", err)
	}
	var missing bool
	err = conn.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM strategy_version WHERE service_id IS NULL)`).Scan(&missing)
	if err != nil {
		return fmt.Errorf("strategy_version: scan service_id: %w", err)
	}
	if missing {
		return fmt.Errorf("strategy_version: rows have no service")
	}
	return nil
}

func dropImageForeignKeys(ctx context.Context, conn postgres.Query, table string) error {
	refs, err := auth.ForeignKeysReferencing(ctx, conn, "service", "image")
	if err != nil {
		return err
	}
	for _, ref := range refs {
		if ref.Table != table {
			continue
		}
		if err = auth.DropConstraint(ctx, conn, table, ref.Name); err != nil {
			return err
		}
	}
	return nil
}

func swapServicePrimaryKey(ctx context.Context, conn postgres.Query) error {
	refs, err := auth.ForeignKeysReferencing(ctx, conn, "service", "image")
	if err != nil {
		return err
	}
	if len(refs) > 0 {
		return fmt.Errorf("service: %s still references service(image)", refs[0].Name)
	}
	if err = auth.EnsurePrimaryKey(ctx, conn, "service", "id"); err != nil {
		return err
	}
	if err = auth.EnsureNotNull(ctx, conn, "service", "image"); err != nil {
		return err
	}
	return auth.EnsureUnique(ctx, conn, "service", "service_owner_id_image_key", "owner_id", "image")
}
