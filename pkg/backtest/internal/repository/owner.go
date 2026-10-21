package repository

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/lhjnilsson/foreverbull/internal/auth"
	"github.com/lhjnilsson/foreverbull/internal/postgres"
	"github.com/lhjnilsson/foreverbull/internal/storage"
)

// IngestionCopier copies an ingestions object from the backtest name onto the
// backtest id. Copy leaves the name key in place. Readable is true when that
// key can be read.
type IngestionCopier interface {
	Readable(ctx context.Context, key string) (bool, error)
	Copy(ctx context.Context, fromKey, toKey string) error
}

// MigrateBacktestOwner assigns owner_id, moves child foreign keys onto
// backtest.id, and copies ingestion objects before the name primary key is
// dropped. A failed copy leaves that primary key in place. It does not search
// for an editor and it does not insert a password. Calling it twice is safe.
// Startup does not call it.
func MigrateBacktestOwner(ctx context.Context, conn *pgxpool.Pool, adminID string, copier IngestionCopier) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if conn == nil {
		return fmt.Errorf("backtest: nil database")
	}
	if err := auth.Migrate(ctx, conn); err != nil {
		return err
	}
	if err := auth.RequireAccount(ctx, conn, "backtest", adminID); err != nil {
		return err
	}
	if copier == nil {
		return fmt.Errorf("%s: ingestion copier is nil", storage.IngestionsBucket)
	}
	exists, err := auth.TableExists(ctx, conn, "backtest")
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("backtest: table is missing")
	}

	done, err := backtestOwnerComplete(ctx, conn)
	if err != nil {
		return err
	}
	if !done {
		if err = migrateBacktestShape(ctx, conn, adminID); err != nil {
			return err
		}
		if err = copyIngestions(ctx, conn, copier); err != nil {
			return err
		}
		if err = swapBacktestPrimaryKey(ctx, conn); err != nil {
			return err
		}
	} else if err = copyIngestions(ctx, conn, copier); err != nil {
		return err
	}
	return auth.Migrate(ctx, conn)
}

func backtestOwnerComplete(ctx context.Context, conn postgres.Query) (bool, error) {
	primaryKey, err := auth.PrimaryKeyColumns(ctx, conn, "backtest")
	if err != nil {
		return false, err
	}
	if !auth.SameColumns(primaryKey, []string{"id"}) {
		return false, nil
	}
	ownerNotNull, err := auth.ColumnNotNull(ctx, conn, "backtest", "owner_id")
	if err != nil || !ownerNotNull {
		return false, err
	}
	unique, err := auth.ConstraintExists(ctx, conn, "backtest", "backtest_owner_id_name_key")
	if err != nil || !unique {
		return false, err
	}
	for _, column := range []struct{ table, column string }{
		{"backtest_period", "backtest_name"},
		{"backtest_period_asset", "backtest_name"},
		{"universe_member", "backtest_name"},
		{"session", "backtest"},
	} {
		exists, err := auth.ColumnExists(ctx, conn, column.table, column.column)
		if err != nil {
			return false, err
		}
		if exists {
			return false, nil
		}
	}
	experiment, err := auth.TableExists(ctx, conn, "strategy_experiment")
	if err != nil {
		return false, err
	}
	if !experiment {
		return true, nil
	}
	nameColumn, err := auth.ColumnExists(ctx, conn, "strategy_experiment", "backtest_name")
	if err != nil || nameColumn {
		return false, err
	}
	return true, nil
}

func migrateBacktestShape(ctx context.Context, conn *pgxpool.Pool, adminID string) error {
	return auth.WithTx(ctx, conn, "backtest", func(tx pgx.Tx) error {
		if err := auth.EnsureSurrogateID(ctx, tx, "backtest"); err != nil {
			return err
		}
		if err := auth.EnsureOwner(ctx, tx, "backtest", adminID); err != nil {
			return err
		}
		if err := repointPeriods(ctx, tx); err != nil {
			return err
		}
		if err := repointUniverse(ctx, tx); err != nil {
			return err
		}
		if err := addBacktestID(ctx, tx, "session", "backtest", true); err != nil {
			return err
		}
		if err := repointExperiment(ctx, tx); err != nil {
			return err
		}
		return dropBacktestNameForeignKeys(ctx, tx)
	})
}

func repointPeriods(ctx context.Context, conn postgres.Query) error {
	if err := addBacktestID(ctx, conn, "backtest_period", "backtest_name", true); err != nil {
		return err
	}
	if err := addBacktestID(ctx, conn, "backtest_period_asset", "backtest_name", false); err != nil {
		return err
	}
	// The asset foreign key still names the old period primary key. Drop it
	// before that primary key can change.
	if err := dropForeignKeysBetween(ctx, conn, "backtest_period_asset", "backtest_period"); err != nil {
		return err
	}
	if err := auth.EnsurePrimaryKey(ctx, conn, "backtest_period", "backtest_id", "start_date", "end_date"); err != nil {
		return err
	}
	if err := auth.EnsurePrimaryKey(ctx, conn, "backtest_period_asset",
		"backtest_id", "start_date", "end_date", "symbol"); err != nil {
		return err
	}
	return auth.EnsureForeignKey(ctx, conn, "backtest_period_asset", "backtest_period_asset_period_fkey",
		`FOREIGN KEY (backtest_id, start_date, end_date) REFERENCES backtest_period (backtest_id, start_date, end_date) ON DELETE CASCADE`)
}

func repointUniverse(ctx context.Context, conn postgres.Query) error {
	if err := addBacktestID(ctx, conn, "universe_member", "backtest_name", true); err != nil {
		return err
	}
	if err := auth.EnsurePrimaryKey(ctx, conn, "universe_member", "backtest_id", "isin", "from_date"); err != nil {
		return err
	}
	return replaceUniverseExclusion(ctx, conn)
}

func repointExperiment(ctx context.Context, conn postgres.Query) error {
	exists, err := auth.TableExists(ctx, conn, "strategy_experiment")
	if err != nil || !exists {
		return err
	}
	return addBacktestID(ctx, conn, "strategy_experiment", "backtest_name", true)
}

func addBacktestID(ctx context.Context, conn postgres.Query, table, nameColumn string, referenceBacktest bool) error {
	exists, err := auth.TableExists(ctx, conn, table)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("%s: table is missing", table)
	}
	quoted, err := auth.QuoteIdent(table)
	if err != nil {
		return err
	}
	quotedName, err := auth.QuoteIdent(nameColumn)
	if err != nil {
		return err
	}
	if _, err = conn.Exec(ctx, fmt.Sprintf(`ALTER TABLE %s ADD COLUMN IF NOT EXISTS backtest_id text`, quoted)); err != nil {
		return fmt.Errorf("%s: backtest_id: %w", table, err)
	}
	hasName, err := auth.ColumnExists(ctx, conn, table, nameColumn)
	if err != nil {
		return err
	}
	if hasName {
		_, err = conn.Exec(ctx, fmt.Sprintf(`
			UPDATE %s AS child
			SET backtest_id = backtest.id
			FROM backtest
			WHERE child.backtest_id IS NULL
			  AND backtest.name = child.%s`, quoted, quotedName))
		if err != nil {
			return fmt.Errorf("%s: backfill backtest_id: %w", table, err)
		}
	}
	var missing bool
	err = conn.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS (SELECT 1 FROM %s WHERE backtest_id IS NULL)`, quoted)).Scan(&missing)
	if err != nil {
		return fmt.Errorf("%s: scan backtest_id: %w", table, err)
	}
	if missing {
		return fmt.Errorf("%s: rows have no backtest", table)
	}
	if err = auth.EnsureNotNull(ctx, conn, table, "backtest_id"); err != nil {
		return err
	}
	if !referenceBacktest {
		return nil
	}
	return auth.EnsureForeignKey(ctx, conn, table, table+"_backtest_id_fkey",
		`FOREIGN KEY (backtest_id) REFERENCES backtest (id) ON DELETE CASCADE`)
}

func replaceUniverseExclusion(ctx context.Context, conn postgres.Query) error {
	if _, err := conn.Exec(ctx, `CREATE EXTENSION IF NOT EXISTS btree_gist`); err != nil {
		return fmt.Errorf("universe_member: btree_gist: %w", err)
	}
	definition, ok, err := auth.ConstraintDefinition(ctx, conn, "universe_member", "universe_member_no_overlap")
	if err != nil {
		return err
	}
	if ok && strings.Contains(definition, "backtest_id") && !strings.Contains(definition, "backtest_name") {
		return nil
	}
	if ok {
		if err = auth.DropConstraint(ctx, conn, "universe_member", "universe_member_no_overlap"); err != nil {
			return err
		}
	}
	_, err = conn.Exec(ctx, `
		ALTER TABLE universe_member
			ADD CONSTRAINT universe_member_no_overlap
			EXCLUDE USING gist (
				backtest_id WITH =,
				isin WITH =,
				daterange(from_date, to_date, '[)') WITH &&
			)`)
	if err != nil {
		return fmt.Errorf("universe_member: exclusion: %w", err)
	}
	return nil
}

func dropForeignKeysBetween(ctx context.Context, conn postgres.Query, source, referenced string) error {
	refs, err := auth.ForeignKeysBetween(ctx, conn, source, referenced)
	if err != nil {
		return err
	}
	for _, ref := range refs {
		if err = auth.DropConstraint(ctx, conn, ref.Table, ref.Name); err != nil {
			return err
		}
	}
	return nil
}

func dropBacktestNameForeignKeys(ctx context.Context, conn postgres.Query) error {
	refs, err := auth.ForeignKeysReferencing(ctx, conn, "backtest", "name")
	if err != nil {
		return err
	}
	for _, ref := range refs {
		if err = auth.DropConstraint(ctx, conn, ref.Table, ref.Name); err != nil {
			return err
		}
	}
	return nil
}

func copyIngestions(ctx context.Context, conn postgres.Query, copier IngestionCopier) error {
	rows, err := conn.Query(ctx, `SELECT id, name FROM backtest ORDER BY name`)
	if err != nil {
		return fmt.Errorf("%s: list backtests: %w", storage.IngestionsBucket, err)
	}
	defer rows.Close()

	type pair struct {
		id   string
		name string
	}
	pending := []pair{}
	for rows.Next() {
		var item pair
		if err = rows.Scan(&item.id, &item.name); err != nil {
			return fmt.Errorf("%s: scan backtest: %w", storage.IngestionsBucket, err)
		}
		pending = append(pending, item)
	}
	if err = rows.Err(); err != nil {
		return fmt.Errorf("%s: list backtests: %w", storage.IngestionsBucket, err)
	}

	for _, item := range pending {
		if err = copyIngestion(ctx, copier, item.name, item.id); err != nil {
			return err
		}
	}
	return nil
}

func copyIngestion(ctx context.Context, copier IngestionCopier, name, id string) error {
	readable, err := copier.Readable(ctx, id)
	if err != nil {
		return fmt.Errorf("%s: read %s: %w", storage.IngestionsBucket, id, err)
	}
	if readable {
		return nil
	}
	hasName, err := copier.Readable(ctx, name)
	if err != nil {
		return fmt.Errorf("%s: read %s: %w", storage.IngestionsBucket, name, err)
	}
	if !hasName {
		return nil
	}
	if err = copier.Copy(ctx, name, id); err != nil {
		return fmt.Errorf("%s: copy %s: %w", storage.IngestionsBucket, name, err)
	}
	readable, err = copier.Readable(ctx, id)
	if err != nil {
		return fmt.Errorf("%s: read %s: %w", storage.IngestionsBucket, id, err)
	}
	if !readable {
		return fmt.Errorf("%s: %s was not readable after copy", storage.IngestionsBucket, id)
	}
	return nil
}

func swapBacktestPrimaryKey(ctx context.Context, conn *pgxpool.Pool) error {
	return auth.WithTx(ctx, conn, "backtest", func(tx pgx.Tx) error {
		if err := auth.EnsurePrimaryKey(ctx, tx, "backtest", "id"); err != nil {
			return err
		}
		if err := auth.EnsureNotNull(ctx, tx, "backtest", "name"); err != nil {
			return err
		}
		if err := auth.EnsureUnique(ctx, tx, "backtest", "backtest_owner_id_name_key", "owner_id", "name"); err != nil {
			return err
		}
		columns := []struct{ table, column string }{
			{"backtest_period", "backtest_name"},
			{"backtest_period_asset", "backtest_name"},
			{"universe_member", "backtest_name"},
			{"session", "backtest"},
			{"strategy_experiment", "backtest_name"},
		}
		for _, column := range columns {
			exists, err := auth.TableExists(ctx, tx, column.table)
			if err != nil {
				return err
			}
			if !exists {
				continue
			}
			if err = auth.DropColumn(ctx, tx, column.table, column.column); err != nil {
				return err
			}
		}
		return nil
	})
}
