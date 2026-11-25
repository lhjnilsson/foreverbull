package repository

import (
	"context"
	"fmt"
	"time"
)

// StoredDividend is one cash dividend stored for a backtest's provider and
// symbols. PayDate is nil when the provider sent none. It is not estimated.
type StoredDividend struct {
	Symbol  string
	ExDate  string
	Amount  float64
	PayDate *string
}

// ListDividends reads the stored dividends of provider and symbols with an
// ex-date from start through end, both days included. The pay date is the
// stored value, or nil. An empty symbol list is no query and no rows.
func (db *Backtest) ListDividends(ctx context.Context, provider string, symbols []string, start, end time.Time,
) ([]StoredDividend, error) {
	if len(symbols) == 0 {
		return []StoredDividend{}, nil
	}
	rows, err := db.Conn.Query(ctx, `
		SELECT symbol,
		       to_char(ex_date, 'YYYY-MM-DD'),
		       amount::float8,
		       to_char(pay_date, 'YYYY-MM-DD')
		FROM dividend
		WHERE provider = $1
		  AND symbol = ANY($2)
		  AND ex_date >= $3::date
		  AND ex_date <= $4::date
		ORDER BY ex_date, symbol`,
		provider, symbols, start.UTC().Format(time.DateOnly), end.UTC().Format(time.DateOnly))
	if err != nil {
		return nil, fmt.Errorf("failed to list dividends: %w", err)
	}
	defer rows.Close()
	found := make([]StoredDividend, 0)
	for rows.Next() {
		var row StoredDividend
		if err := rows.Scan(&row.Symbol, &row.ExDate, &row.Amount, &row.PayDate); err != nil {
			return nil, fmt.Errorf("failed to read dividend: %w", err)
		}
		found = append(found, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to list dividends: %w", err)
	}
	return found, nil
}
