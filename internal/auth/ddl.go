package auth

import (
	"context"
	"fmt"
	"strings"

	"github.com/lhjnilsson/foreverbull/internal/postgres"
)

// EnsureSurrogateID adds id, fills uuid_generate_v4() where null, and makes it unique.
// Duplicate or null ids are an error. Rows are not deleted.
func EnsureSurrogateID(ctx context.Context, conn postgres.Query, table string) error {
	quoted, err := QuoteIdent(table)
	if err != nil {
		return err
	}
	if _, err = conn.Exec(ctx, fmt.Sprintf(`ALTER TABLE %s ADD COLUMN IF NOT EXISTS id text`, quoted)); err != nil {
		return fmt.Errorf("%s: id column: %w", table, err)
	}
	if _, err = conn.Exec(ctx, fmt.Sprintf(`UPDATE %s SET id = uuid_generate_v4()::text WHERE id IS NULL`, quoted)); err != nil {
		return fmt.Errorf("%s: backfill id: %w", table, err)
	}

	var bad bool
	err = conn.QueryRow(ctx, fmt.Sprintf(`
		SELECT EXISTS (SELECT 1 FROM %s WHERE id IS NULL)
			OR EXISTS (SELECT 1 FROM %s GROUP BY id HAVING COUNT(*) > 1)`, quoted, quoted)).Scan(&bad)
	if err != nil {
		return fmt.Errorf("%s: scan id: %w", table, err)
	}
	if bad {
		return fmt.Errorf("%s: rows violate unique id", table)
	}
	if _, err = conn.Exec(ctx, fmt.Sprintf(`ALTER TABLE %s ALTER COLUMN id SET DEFAULT uuid_generate_v4()::text`, quoted)); err != nil {
		return fmt.Errorf("%s: id default: %w", table, err)
	}
	if _, err = conn.Exec(ctx, fmt.Sprintf(`ALTER TABLE %s ALTER COLUMN id SET NOT NULL`, quoted)); err != nil {
		return fmt.Errorf("%s: id not null: %w", table, err)
	}
	return EnsureUnique(ctx, conn, table, table+"_id_key", "id")
}

// EnsureOwner adds owner_id, assigns null rows to adminID, then sets NOT NULL.
// It does not insert a password and it does not delete domain rows.
func EnsureOwner(ctx context.Context, conn postgres.Query, table, adminID string) error {
	quoted, err := QuoteIdent(table)
	if err != nil {
		return err
	}
	exists, err := ColumnExists(ctx, conn, table, "owner_id")
	if err != nil {
		return err
	}
	if !exists {
		if _, err = conn.Exec(ctx, fmt.Sprintf(`ALTER TABLE %s ADD COLUMN owner_id text`, quoted)); err != nil {
			return fmt.Errorf("%s: owner_id: %w", table, err)
		}
	}
	if err = EnsureForeignKey(ctx, conn, table, table+"_owner_id_fkey",
		`FOREIGN KEY (owner_id) REFERENCES account (id) ON DELETE RESTRICT`); err != nil {
		return err
	}
	if _, err = conn.Exec(ctx, fmt.Sprintf(`UPDATE %s SET owner_id = $1 WHERE owner_id IS NULL`, quoted), adminID); err != nil {
		return fmt.Errorf("%s: assign owner: %w", table, err)
	}
	if _, err = conn.Exec(ctx, fmt.Sprintf(`ALTER TABLE %s ALTER COLUMN owner_id SET NOT NULL`, quoted)); err != nil {
		return fmt.Errorf("%s: owner_id not null: %w", table, err)
	}
	return nil
}

// EnsureUnique adds a unique constraint unless that name, or a primary key on
// the same columns, is already present.
func EnsureUnique(ctx context.Context, conn postgres.Query, table, name string, columns ...string) error {
	exists, err := ConstraintExists(ctx, conn, table, name)
	if err != nil || exists {
		return err
	}
	primaryKey, err := PrimaryKeyColumns(ctx, conn, table)
	if err != nil {
		return err
	}
	if SameColumns(primaryKey, columns) {
		return nil
	}

	quotedTable, err := QuoteIdent(table)
	if err != nil {
		return err
	}
	quotedName, err := QuoteIdent(name)
	if err != nil {
		return err
	}
	quotedColumns := make([]string, len(columns))
	for i, column := range columns {
		quotedColumns[i], err = QuoteIdent(column)
		if err != nil {
			return err
		}
	}
	_, err = conn.Exec(ctx, fmt.Sprintf(
		`ALTER TABLE %s ADD CONSTRAINT %s UNIQUE (%s)`,
		quotedTable, quotedName, strings.Join(quotedColumns, ", ")))
	if err != nil {
		return fmt.Errorf("%s: %s: %w", table, name, err)
	}
	return nil
}

// EnsureForeignKey adds a foreign key when the constraint name is absent.
// definition is the constraint body, for example "FOREIGN KEY (id) REFERENCES account (id)".
func EnsureForeignKey(ctx context.Context, conn postgres.Query, table, name, definition string) error {
	exists, err := ConstraintExists(ctx, conn, table, name)
	if err != nil || exists {
		return err
	}
	quotedTable, err := QuoteIdent(table)
	if err != nil {
		return err
	}
	quotedName, err := QuoteIdent(name)
	if err != nil {
		return err
	}
	_, err = conn.Exec(ctx, fmt.Sprintf(`ALTER TABLE %s ADD CONSTRAINT %s %s`, quotedTable, quotedName, definition))
	if err != nil {
		return fmt.Errorf("%s: %s: %w", table, name, err)
	}
	return nil
}

// EnsureNotNull sets the column NOT NULL.
func EnsureNotNull(ctx context.Context, conn postgres.Query, table, column string) error {
	quotedTable, err := QuoteIdent(table)
	if err != nil {
		return err
	}
	quotedColumn, err := QuoteIdent(column)
	if err != nil {
		return err
	}
	if _, err = conn.Exec(ctx, fmt.Sprintf(`ALTER TABLE %s ALTER COLUMN %s SET NOT NULL`, quotedTable, quotedColumn)); err != nil {
		return fmt.Errorf("%s: %s not null: %w", table, column, err)
	}
	return nil
}

// EnsurePrimaryKey makes the primary key those columns.
// An existing primary key on different columns is dropped first.
func EnsurePrimaryKey(ctx context.Context, conn postgres.Query, table string, columns ...string) error {
	current, err := PrimaryKeyColumns(ctx, conn, table)
	if err != nil {
		return err
	}
	if SameColumns(current, columns) {
		return nil
	}
	if len(current) > 0 {
		if err = DropPrimaryKey(ctx, conn, table); err != nil {
			return err
		}
	}
	quotedTable, err := QuoteIdent(table)
	if err != nil {
		return err
	}
	quotedColumns := make([]string, len(columns))
	for i, column := range columns {
		quotedColumns[i], err = QuoteIdent(column)
		if err != nil {
			return err
		}
	}
	if _, err = conn.Exec(ctx, fmt.Sprintf(
		`ALTER TABLE %s ADD PRIMARY KEY (%s)`,
		quotedTable, strings.Join(quotedColumns, ", "))); err != nil {
		return fmt.Errorf("%s: primary key: %w", table, err)
	}
	return nil
}

// DropColumn drops column when it is present.
func DropColumn(ctx context.Context, conn postgres.Query, table, column string) error {
	exists, err := ColumnExists(ctx, conn, table, column)
	if err != nil || !exists {
		return err
	}
	quotedTable, err := QuoteIdent(table)
	if err != nil {
		return err
	}
	quotedColumn, err := QuoteIdent(column)
	if err != nil {
		return err
	}
	if _, err = conn.Exec(ctx, fmt.Sprintf(`ALTER TABLE %s DROP COLUMN %s`, quotedTable, quotedColumn)); err != nil {
		return fmt.Errorf("%s: drop %s: %w", table, column, err)
	}
	return nil
}
