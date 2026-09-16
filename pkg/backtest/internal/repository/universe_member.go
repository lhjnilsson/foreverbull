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
	PRIMARY KEY (backtest_name, isin, from_date)
);
`

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
