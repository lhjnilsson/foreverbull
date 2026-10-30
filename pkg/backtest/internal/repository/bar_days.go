package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// BarDays reads a symbol's stored bars in the finance service's ohlc table
// for the missing-session check. Like the ingester, it does not count a row
// with a price of 0 or less as a bar.
type BarDays struct {
	Conn *pgxpool.Pool
}

// SymbolBars is one symbol's span and the bar days inside a window.
type SymbolBars struct {
	// First and Last are the symbol's first and last bar over all stored
	// time, as UTC dates, so a window that starts or ends inside the span
	// still sees the sessions missing at its edges.
	First, Last time.Time
	// Days are the bar days inside the window, oldest first.
	Days []time.Time
}

// Fixed queries, so no caller can put its own text into the SQL.
const (
	barSpanQuery = `
	SELECT symbol, min(time), max(time)
	  FROM ohlc
	 WHERE provider = $1
	   AND symbol = ANY($2)
	   AND open > 0 AND high > 0 AND low > 0 AND close > 0
	 GROUP BY symbol`
	// Whole UTC days from start to end, so a bar stamped at the session
	// open on the end date counts: the span already reaches past it.
	barDaysQuery = `
	SELECT symbol, time
	  FROM ohlc
	 WHERE provider = $1
	   AND symbol = ANY($2)
	   AND time >= $3::date AND time < $4::date + 1
	   AND open > 0 AND high > 0 AND low > 0 AND close > 0
	 ORDER BY symbol, time`
)

// List returns each symbol's span and its bar days from start to end, as
// UTC midnights. A symbol with no bars has no entry.
func (db *BarDays) List(ctx context.Context, provider string, symbols []string, start, end time.Time) (map[string]*SymbolBars, error) {
	bars, err := db.spans(ctx, provider, symbols)
	if err != nil {
		return nil, err
	}
	rows, err := db.Conn.Query(ctx, barDaysQuery, provider, symbols, dateString(start), dateString(end))
	if err != nil {
		return nil, fmt.Errorf("failed to query bar days: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var symbol string
		var at time.Time
		if err := rows.Scan(&symbol, &at); err != nil {
			return nil, fmt.Errorf("failed to scan bar day: %w", err)
		}
		entry, ok := bars[symbol]
		if !ok {
			continue
		}
		day := dateOnly(at.UTC())
		if n := len(entry.Days); n > 0 && entry.Days[n-1].Equal(day) {
			continue
		}
		entry.Days = append(entry.Days, day)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to query bar days: %w", err)
	}
	return bars, nil
}

func (db *BarDays) spans(ctx context.Context, provider string, symbols []string) (map[string]*SymbolBars, error) {
	rows, err := db.Conn.Query(ctx, barSpanQuery, provider, symbols)
	if err != nil {
		return nil, fmt.Errorf("failed to query bar spans: %w", err)
	}
	defer rows.Close()
	bars := make(map[string]*SymbolBars)
	for rows.Next() {
		var symbol string
		var first, last time.Time
		if err := rows.Scan(&symbol, &first, &last); err != nil {
			return nil, fmt.Errorf("failed to scan bar span: %w", err)
		}
		bars[symbol] = &SymbolBars{First: dateOnly(first.UTC()), Last: dateOnly(last.UTC())}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to query bar spans: %w", err)
	}
	return bars, nil
}
