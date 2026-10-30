package repository

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// IDForSession returns the backtest id stored on a session row.
func (db *Backtest) IDForSession(ctx context.Context, sessionID string) (string, error) {
	if sessionID == "" {
		return "", fmt.Errorf("session id is empty")
	}
	var id string
	err := db.Conn.QueryRow(ctx, `SELECT backtest_id FROM session WHERE id = $1`, sessionID).Scan(&id)
	if err != nil {
		return "", err
	}
	return id, nil
}

// UpdatePeriodAssetByID disables or enables one symbol on one backtest id.
func (db *Backtest) UpdatePeriodAssetByID(ctx context.Context, backtestID, symbol string, enabled bool) error {
	if backtestID == "" {
		return fmt.Errorf("backtest id is empty")
	}
	_, err := db.Conn.Exec(ctx, `
		UPDATE backtest_period_asset SET enabled = $3
		WHERE backtest_id = $1 AND symbol = $2`, backtestID, symbol, enabled)
	if err != nil {
		return fmt.Errorf("failed to update backtest period asset: %w", err)
	}
	return nil
}

// DisablePeriodAssetsByID disables every period asset of one backtest id
// whose symbol is in symbols, in one statement, and returns the rows it
// changed.
func (db *Backtest) DisablePeriodAssetsByID(ctx context.Context, backtestID string, symbols []string) (int64, error) {
	if backtestID == "" {
		return 0, fmt.Errorf("backtest id is empty")
	}
	tag, err := db.Conn.Exec(ctx,
		`UPDATE backtest_period_asset SET enabled = false WHERE backtest_id = $1 AND symbol = ANY($2)`,
		backtestID, symbols)
	if err != nil {
		return 0, fmt.Errorf("failed to disable backtest period assets: %w", err)
	}
	return tag.RowsAffected(), nil
}

// ReplaceByID overwrites membership intervals for one backtest id.
func (db *UniverseMember) ReplaceByID(ctx context.Context, backtestID string, members []MemberInterval) error {
	if backtestID == "" {
		return fmt.Errorf("backtest id is empty")
	}
	if _, err := db.Conn.Exec(ctx, `DELETE FROM universe_member WHERE backtest_id = $1`, backtestID); err != nil {
		return fmt.Errorf("failed to clear universe_member: %w", err)
	}
	for _, member := range members {
		if err := insertMemberID(ctx, db, backtestID, member); err != nil {
			return err
		}
	}
	return nil
}

func insertMemberID(ctx context.Context, db *UniverseMember, backtestID string, member MemberInterval) error {
	isin := strings.TrimSpace(member.ISIN)
	if isin == "" {
		return fmt.Errorf("missing ISIN")
	}
	var to any
	if member.ToDate != nil {
		to = dateString(*member.ToDate)
	}
	_, err := db.Conn.Exec(ctx, `
		INSERT INTO universe_member (backtest_id, isin, from_date, to_date)
		VALUES ($1, $2, $3::date, $4::date)`,
		backtestID, isin, dateString(member.FromDate), to)
	if err != nil {
		return fmt.Errorf("failed to insert universe_member: %w", err)
	}
	return nil
}

// ListByID returns membership intervals for one backtest id.
func (db *UniverseMember) ListByID(ctx context.Context, backtestID string) ([]MemberInterval, error) {
	rows, err := db.Conn.Query(ctx, `
		SELECT isin, from_date, to_date
		  FROM universe_member
		 WHERE backtest_id = $1
		 ORDER BY isin, from_date`, backtestID)
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

// SymbolsAtID returns tickers for members of one backtest id on date t.
func (db *UniverseMember) SymbolsAtID(ctx context.Context, backtestID string, t time.Time) ([]string, error) {
	rows, err := db.Conn.Query(ctx, `
		SELECT s.symbol
		  FROM universe_member m
		  JOIN asset_symbol s ON s.isin = m.isin
		 WHERE m.backtest_id = $1
		   AND m.from_date <= $2::date
		   AND (m.to_date IS NULL OR $2::date < m.to_date)
		   AND s.from_date <= $2::date
		   AND (s.to_date IS NULL OR $2::date < s.to_date)
		 ORDER BY s.symbol`,
		backtestID, dateString(t))
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
		symbols = append(symbols, symbol)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to query member symbols: %w", err)
	}
	return symbols, nil
}
