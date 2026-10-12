package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/lhjnilsson/foreverbull/internal/postgres"
)

const UniverseMemberTable = `
CREATE TABLE IF NOT EXISTS universe_member (
	backtest_name text REFERENCES backtest (name) ON DELETE CASCADE,
	isin text NOT NULL,
	from_date date NOT NULL,
	to_date date,
	PRIMARY KEY (backtest_name, isin, from_date),
	CONSTRAINT universe_member_isin_fkey FOREIGN KEY (isin) REFERENCES instrument (isin),
	CONSTRAINT universe_member_no_overlap EXCLUDE USING gist (
		backtest_name WITH =,
		isin WITH =,
		daterange(from_date, to_date, '[)') WITH &&
	)
);
`

// MigrateUniverseMember adds the instrument foreign key and the per-backtest
// half-open exclusion. Rows that would violate the new constraint stay.
// Calling it twice is safe.
func MigrateUniverseMember(ctx context.Context, conn postgres.Query) error {
	if _, err := conn.Exec(ctx, `CREATE EXTENSION IF NOT EXISTS btree_gist`); err != nil {
		return fmt.Errorf("failed to create extension btree_gist: %w", err)
	}
	if err := migrateUniverseMemberForeignKey(ctx, conn); err != nil {
		return err
	}
	return migrateUniverseMemberExclusion(ctx, conn)
}

func migrateUniverseMemberForeignKey(ctx context.Context, conn postgres.Query) error {
	exists, err := tableConstraintExists(ctx, conn, "universe_member", "universe_member_isin_fkey")
	if err != nil {
		return err
	}
	if exists {
		return nil
	}

	var bad bool
	err = conn.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM universe_member m
			LEFT JOIN instrument i ON i.isin = m.isin
			WHERE i.isin IS NULL
		)`).Scan(&bad)
	if err != nil {
		return fmt.Errorf("universe_member: scan foreign key: %w", err)
	}
	if bad {
		return fmt.Errorf("universe_member: rows violate foreign key isin references instrument(isin)")
	}

	_, err = conn.Exec(ctx, `
		ALTER TABLE universe_member
			ADD CONSTRAINT universe_member_isin_fkey
			FOREIGN KEY (isin) REFERENCES instrument (isin)`)
	if err != nil {
		return fmt.Errorf("universe_member: foreign key: %w", err)
	}
	return nil
}

func migrateUniverseMemberExclusion(ctx context.Context, conn postgres.Query) error {
	exists, err := tableConstraintExists(ctx, conn, "universe_member", "universe_member_no_overlap")
	if err != nil {
		return err
	}
	if exists {
		return nil
	}

	var bad bool
	err = conn.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM universe_member a
			JOIN universe_member b
			  ON a.backtest_name = b.backtest_name
			 AND a.isin = b.isin
			 AND a.from_date < b.from_date
			 AND daterange(a.from_date, a.to_date, '[)') && daterange(b.from_date, b.to_date, '[)')
		)`).Scan(&bad)
	if err != nil {
		return fmt.Errorf("universe_member: scan overlap: %w", err)
	}
	if bad {
		return fmt.Errorf("universe_member: rows violate exclusion on backtest_name, isin, and daterange(from_date, to_date, '[)')")
	}

	_, err = conn.Exec(ctx, `
		ALTER TABLE universe_member
			ADD CONSTRAINT universe_member_no_overlap
			EXCLUDE USING gist (
				backtest_name WITH =,
				isin WITH =,
				daterange(from_date, to_date, '[)') WITH &&
			)`)
	if err != nil {
		return fmt.Errorf("universe_member: exclusion: %w", err)
	}
	return nil
}

// MemberInterval is a half-open [FromDate, ToDate) row in universe_member.
// Nil ToDate means still a member at the backtest end.
type MemberInterval struct {
	ISIN     string
	FromDate time.Time
	ToDate   *time.Time
}

// UniverseMember stores point-in-time ETF membership keyed by ISIN.
type UniverseMember struct {
	Conn postgres.Query
}

func dateOnly(t time.Time) time.Time {
	year, month, day := t.Date()
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}

func dateString(t time.Time) string {
	return dateOnly(t).Format("2006-01-02")
}

// Replace overwrites membership intervals for a backtest. Yearly period lists
// are not written; this table is the membership source of truth.
func (db *UniverseMember) Replace(ctx context.Context, backtestName string, members []MemberInterval) error {
	if _, err := db.Conn.Exec(ctx, `DELETE FROM universe_member WHERE backtest_name=$1`, backtestName); err != nil {
		return fmt.Errorf("failed to clear universe_member: %w", err)
	}

	for _, member := range members {
		isin := strings.TrimSpace(member.ISIN)
		if isin == "" {
			return fmt.Errorf("missing ISIN")
		}

		var to any
		if member.ToDate != nil {
			to = dateString(*member.ToDate)
		}

		_, err := db.Conn.Exec(ctx, `
			INSERT INTO universe_member (backtest_name, isin, from_date, to_date)
			VALUES ($1, $2, $3::date, $4::date)`,
			backtestName, isin, dateString(member.FromDate), to)
		if err != nil {
			return fmt.Errorf("failed to insert universe_member: %w", err)
		}
	}

	return nil
}

// List returns all membership intervals for a backtest, ordered by ISIN then from_date.
func (db *UniverseMember) List(ctx context.Context, backtestName string) ([]MemberInterval, error) {
	rows, err := db.Conn.Query(ctx, `
		SELECT isin, from_date, to_date
		  FROM universe_member
		 WHERE backtest_name = $1
		 ORDER BY isin, from_date`, backtestName)
	if err != nil {
		return nil, fmt.Errorf("failed to list universe_member: %w", err)
	}
	defer rows.Close()

	members := make([]MemberInterval, 0)
	for rows.Next() {
		var member MemberInterval
		if err := rows.Scan(&member.ISIN, &member.FromDate, &member.ToDate); err != nil {
			return nil, fmt.Errorf("failed to scan universe_member: %w", err)
		}
		member.FromDate = dateOnly(member.FromDate)
		if member.ToDate != nil {
			to := dateOnly(*member.ToDate)
			member.ToDate = &to
		}
		members = append(members, member)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to list universe_member: %w", err)
	}

	return members, nil
}

// At returns ISINs that are members of the backtest on date t ([from, to)).
func (db *UniverseMember) At(ctx context.Context, backtestName string, t time.Time) ([]string, error) {
	rows, err := db.Conn.Query(ctx, `
		SELECT isin FROM universe_member
		 WHERE backtest_name = $1
		   AND from_date <= $2::date
		   AND (to_date IS NULL OR $2::date < to_date)
		 ORDER BY isin`,
		backtestName, dateString(t))
	if err != nil {
		return nil, fmt.Errorf("failed to query universe_member: %w", err)
	}
	defer rows.Close()

	isins := make([]string, 0)
	for rows.Next() {
		var isin string
		if err := rows.Scan(&isin); err != nil {
			return nil, fmt.Errorf("failed to scan universe_member isin: %w", err)
		}
		isins = append(isins, isin)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to query universe_member: %w", err)
	}

	return isins, nil
}

// SymbolsAt returns the ticker of each member on date t via asset_symbol.
// Ticker changes do not drop a name; a missing alias omits that ISIN.
func (db *UniverseMember) SymbolsAt(ctx context.Context, backtestName string, t time.Time) ([]string, error) {
	rows, err := db.Conn.Query(ctx, `
		SELECT s.symbol
		  FROM universe_member m
		  JOIN asset_symbol s ON s.isin = m.isin
		 WHERE m.backtest_name = $1
		   AND m.from_date <= $2::date
		   AND (m.to_date IS NULL OR $2::date < m.to_date)
		   AND s.from_date <= $2::date
		   AND (s.to_date IS NULL OR $2::date < s.to_date)
		 ORDER BY s.symbol`,
		backtestName, dateString(t))
	if err != nil {
		return nil, fmt.Errorf("failed to query member symbols: %w", err)
	}
	defer rows.Close()

	symbols := make([]string, 0)
	for rows.Next() {
		var symbol string
		if err := rows.Scan(&symbol); err != nil {
			return nil, fmt.Errorf("failed to scan member symbol: %w", err)
		}
		if symbol == "" {
			continue
		}
		symbols = append(symbols, symbol)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to query member symbols: %w", err)
	}

	return symbols, nil
}
