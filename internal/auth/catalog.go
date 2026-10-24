package auth

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/lhjnilsson/foreverbull/internal/postgres"
)

var identPattern = regexp.MustCompile(`^[a-z_][a-z0-9_]*$`)

// ConstraintRef is a foreign key read from pg_constraint.
type ConstraintRef struct {
	Name  string
	Table string
}

// QuoteIdent quotes a lowercase catalog name. Names that are not identifiers
// are rejected so a migration cannot interpolate arbitrary SQL.
func QuoteIdent(name string) (string, error) {
	if !identPattern.MatchString(name) {
		return "", fmt.Errorf("invalid identifier %q", name)
	}
	return pgx.Identifier{name}.Sanitize(), nil
}

// RequireAccount reports a missing admin id. It looks the id up and does not
// read account.role, insert a password, or change a domain row.
func RequireAccount(ctx context.Context, conn postgres.Query, table, accountID string) error {
	if conn == nil {
		return fmt.Errorf("%s: nil database", table)
	}
	if strings.TrimSpace(accountID) == "" {
		return fmt.Errorf("%s: admin account id is required", table)
	}

	var exists bool
	err := conn.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM account WHERE id = $1)`, accountID).Scan(&exists)
	if err != nil {
		return fmt.Errorf("%s: admin account: %w", table, err)
	}
	if !exists {
		return fmt.Errorf("%s: admin account id is missing", table)
	}
	return nil
}

// WithTx runs fn in one transaction. Postgres DDL rolls back with the transaction.
func WithTx(ctx context.Context, db DB, table string, fn func(pgx.Tx) error) error {
	if db == nil {
		return fmt.Errorf("%s: nil database", table)
	}
	tx, err := db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("%s: begin: %w", table, err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err = fn(tx); err != nil {
		return err
	}
	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("%s: commit: %w", table, err)
	}
	return nil
}

// TableExists reports whether table is a table in the current schema.
func TableExists(ctx context.Context, conn postgres.Query, table string) (bool, error) {
	var exists bool
	err := conn.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM information_schema.tables
			WHERE table_schema = current_schema()
			  AND table_name = $1
			  AND table_type = 'BASE TABLE'
		)`, table).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("%s: read table: %w", table, err)
	}
	return exists, nil
}

// ColumnExists reports whether column is present. A missing table is false.
func ColumnExists(ctx context.Context, conn postgres.Query, table, column string) (bool, error) {
	var exists bool
	err := conn.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM information_schema.columns
			WHERE table_schema = current_schema()
			  AND table_name = $1
			  AND column_name = $2
		)`, table, column).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("%s: read column %s: %w", table, column, err)
	}
	return exists, nil
}

// ColumnNotNull reports whether column is NOT NULL. A missing column is false.
func ColumnNotNull(ctx context.Context, conn postgres.Query, table, column string) (bool, error) {
	var notNull bool
	err := conn.QueryRow(ctx, `
		SELECT COALESCE(c.is_nullable = 'NO', false)
		FROM (SELECT 1) AS seed
		LEFT JOIN information_schema.columns c
			ON c.table_schema = current_schema()
			AND c.table_name = $1
			AND c.column_name = $2`, table, column).Scan(&notNull)
	if err != nil {
		return false, fmt.Errorf("%s: read %s nullability: %w", table, column, err)
	}
	return notNull, nil
}

// ConstraintExists reports whether the named constraint is on table.
func ConstraintExists(ctx context.Context, conn postgres.Query, table, name string) (bool, error) {
	var exists bool
	err := conn.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM pg_constraint c
			JOIN pg_class t ON t.oid = c.conrelid
			JOIN pg_namespace n ON n.oid = t.relnamespace
			WHERE n.nspname = current_schema()
			  AND t.relname = $1
			  AND c.conname = $2
		)`, table, name).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("%s: read constraint %s: %w", table, name, err)
	}
	return exists, nil
}

// ConstraintDefinition returns the constraint definition. The bool is false when it is absent.
func ConstraintDefinition(ctx context.Context, conn postgres.Query, table, name string) (string, bool, error) {
	var def string
	err := conn.QueryRow(ctx, `
		SELECT pg_get_constraintdef(c.oid)
		FROM pg_constraint c
		JOIN pg_class t ON t.oid = c.conrelid
		JOIN pg_namespace n ON n.oid = t.relnamespace
		WHERE n.nspname = current_schema()
		  AND t.relname = $1
		  AND c.conname = $2`, table, name).Scan(&def)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("%s: read constraint %s: %w", table, name, err)
	}
	return def, true, nil
}

// PrimaryKeyColumns returns the primary-key columns in key order.
// An absent primary key is an empty slice.
func PrimaryKeyColumns(ctx context.Context, conn postgres.Query, table string) ([]string, error) {
	rows, err := conn.Query(ctx, `
		SELECT a.attname
		FROM pg_constraint c
		JOIN pg_class t ON t.oid = c.conrelid
		JOIN pg_namespace n ON n.oid = t.relnamespace
		JOIN LATERAL unnest(c.conkey) WITH ORDINALITY AS cols(attnum, ord) ON true
		JOIN pg_attribute a ON a.attrelid = t.oid AND a.attnum = cols.attnum
		WHERE n.nspname = current_schema()
		  AND t.relname = $1
		  AND c.contype = 'p'
		ORDER BY cols.ord`, table)
	if err != nil {
		return nil, fmt.Errorf("%s: read primary key: %w", table, err)
	}
	defer rows.Close()

	columns := []string{}
	for rows.Next() {
		var column string
		if err = rows.Scan(&column); err != nil {
			return nil, fmt.Errorf("%s: scan primary key: %w", table, err)
		}
		columns = append(columns, column)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("%s: primary key: %w", table, err)
	}
	return columns, nil
}

// ForeignKeysReferencing lists foreign keys whose referenced column is refColumn on refTable.
func ForeignKeysReferencing(ctx context.Context, conn postgres.Query, refTable, refColumn string) ([]ConstraintRef, error) {
	rows, err := conn.Query(ctx, `
		SELECT DISTINCT c.conname, src.relname
		FROM pg_constraint c
		JOIN pg_class src ON src.oid = c.conrelid
		JOIN pg_namespace sn ON sn.oid = src.relnamespace
		JOIN pg_class dst ON dst.oid = c.confrelid
		JOIN pg_namespace dn ON dn.oid = dst.relnamespace
		JOIN LATERAL unnest(c.confkey) AS ref(attnum) ON true
		JOIN pg_attribute a ON a.attrelid = dst.oid AND a.attnum = ref.attnum
		WHERE c.contype = 'f'
		  AND sn.nspname = current_schema()
		  AND dn.nspname = current_schema()
		  AND dst.relname = $1
		  AND a.attname = $2
		ORDER BY src.relname, c.conname`, refTable, refColumn)
	if err != nil {
		return nil, fmt.Errorf("%s: read foreign keys to %s: %w", refTable, refColumn, err)
	}
	defer rows.Close()

	refs := []ConstraintRef{}
	for rows.Next() {
		var ref ConstraintRef
		if err = rows.Scan(&ref.Name, &ref.Table); err != nil {
			return nil, fmt.Errorf("%s: scan foreign keys: %w", refTable, err)
		}
		refs = append(refs, ref)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("%s: foreign keys: %w", refTable, err)
	}
	return refs, nil
}

// ForeignKeysBetween lists foreign keys from sourceTable to refTable.
func ForeignKeysBetween(ctx context.Context, conn postgres.Query, sourceTable, refTable string) ([]ConstraintRef, error) {
	rows, err := conn.Query(ctx, `
		SELECT c.conname, src.relname
		FROM pg_constraint c
		JOIN pg_class src ON src.oid = c.conrelid
		JOIN pg_namespace sn ON sn.oid = src.relnamespace
		JOIN pg_class dst ON dst.oid = c.confrelid
		JOIN pg_namespace dn ON dn.oid = dst.relnamespace
		WHERE c.contype = 'f'
		  AND sn.nspname = current_schema()
		  AND dn.nspname = current_schema()
		  AND src.relname = $1
		  AND dst.relname = $2
		ORDER BY c.conname`, sourceTable, refTable)
	if err != nil {
		return nil, fmt.Errorf("%s: read foreign keys to %s: %w", sourceTable, refTable, err)
	}
	defer rows.Close()

	refs := []ConstraintRef{}
	for rows.Next() {
		var ref ConstraintRef
		if err = rows.Scan(&ref.Name, &ref.Table); err != nil {
			return nil, fmt.Errorf("%s: scan foreign keys: %w", sourceTable, err)
		}
		refs = append(refs, ref)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("%s: foreign keys: %w", sourceTable, err)
	}
	return refs, nil
}

// SameColumns reports whether the two column lists are the same key.
func SameColumns(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

// DropPrimaryKey drops the primary key when one exists.
// A missing primary key is success.
func DropPrimaryKey(ctx context.Context, conn postgres.Query, table string) error {
	var name string
	err := conn.QueryRow(ctx, `
		SELECT c.conname
		FROM pg_constraint c
		JOIN pg_class t ON t.oid = c.conrelid
		JOIN pg_namespace n ON n.oid = t.relnamespace
		WHERE n.nspname = current_schema()
		  AND t.relname = $1
		  AND c.contype = 'p'`, table).Scan(&name)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		return fmt.Errorf("%s: read primary key: %w", table, err)
	}
	return DropConstraint(ctx, conn, table, name)
}

// DropConstraint drops a constraint. The name must be a catalog identifier.
func DropConstraint(ctx context.Context, conn postgres.Query, table, name string) error {
	quotedTable, err := QuoteIdent(table)
	if err != nil {
		return err
	}
	quotedName, err := QuoteIdent(name)
	if err != nil {
		return fmt.Errorf("%s: constraint %q: %w", table, name, err)
	}
	if _, err = conn.Exec(ctx, fmt.Sprintf(`ALTER TABLE %s DROP CONSTRAINT %s`, quotedTable, quotedName)); err != nil {
		return fmt.Errorf("%s: drop %s: %w", table, name, err)
	}
	return nil
}
