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

// Ticker is a symbol whose bars are read only from From up to, not
// including, To, both UTC dates. A zero From or To leaves that side open.
type Ticker struct {
	Symbol   string
	From, To time.Time
}

// Fixed queries, so no caller can put its own text into the SQL. Each
// ticker's bars are read only inside its own window: $2, $3 and $4 are the
// symbols and their From and To dates, NULL for an open side.
const (
	barWindows = `
	WITH w AS (
		SELECT symbol,
		       lo::timestamp AT TIME ZONE 'UTC' AS lo,
		       hi::timestamp AT TIME ZONE 'UTC' AS hi
		  FROM unnest($2::text[], $3::text[]::date[], $4::text[]::date[]) AS w(symbol, lo, hi))`
	barInWindow = `
	  FROM w
	  JOIN ohlc o ON o.symbol = w.symbol
	 WHERE o.provider = $1
	   AND (w.lo IS NULL OR o.time >= w.lo)
	   AND (w.hi IS NULL OR o.time < w.hi)
	   AND o.open > 0 AND o.high > 0 AND o.low > 0 AND o.close > 0`
	barSpanQuery = barWindows + `
	SELECT w.symbol, min(o.time), max(o.time)` + barInWindow + `
	 GROUP BY w.symbol`
	// Whole UTC days from start to end, so a bar stamped at the session
	// open on the end date counts: the span already reaches past it.
	barDaysQuery = barWindows + `
	SELECT w.symbol, o.time` + barInWindow + `
	   AND o.time >= $5::date AND o.time < $6::date + 1
	 ORDER BY w.symbol, o.time`
)

// tickersQuery lists the tickers of the instrument each symbol belongs to,
// oldest first, with the dates each was that instrument's ticker: a bar of
// the instrument may be stored under any of them (universe.md §7.4). A
// symbol held by more than one instrument is never part of a group, neither
// as the symbol asked for, which then has no entry and is read alone, nor
// as another ticker of an instrument. A NULL to_date means still in use.
const tickersQuery = `
	WITH owner AS (
		SELECT symbol, min(isin) AS isin
		  FROM asset_symbol
		 WHERE symbol = ANY($1)
		 GROUP BY symbol
		HAVING count(DISTINCT isin) = 1)
	SELECT o.symbol, s.symbol, min(s.from_date) AS since,
	       CASE WHEN bool_or(s.to_date IS NULL) THEN NULL ELSE max(s.to_date) END
	  FROM owner o
	  JOIN asset_symbol s ON s.isin = o.isin
	 WHERE NOT EXISTS (
		SELECT 1
		  FROM asset_symbol x
		 WHERE x.symbol = s.symbol
		   AND x.isin <> s.isin)
	 GROUP BY o.symbol, s.symbol
	 ORDER BY o.symbol, since, s.symbol`

// Tickers returns, for each symbol that belongs to one instrument in
// asset_symbol, that instrument's tickers held by no other instrument,
// oldest first, each with the dates it was the instrument's ticker. The
// symbol is one of them. The latest ticker also covers the instrument's
// earlier dates, since a provider keeps a renamed instrument's history
// under its current ticker (Yahoo keeps FB's under META); no other
// instrument ever held it. A symbol with no row, or with rows for two
// instruments, has no entry.
func (db *BarDays) Tickers(ctx context.Context, symbols []string) (map[string][]Ticker, error) {
	rows, err := db.Conn.Query(ctx, tickersQuery, symbols)
	if err != nil {
		return nil, fmt.Errorf("failed to query tickers: %w", err)
	}
	defer rows.Close()
	tickers := make(map[string][]Ticker)
	for rows.Next() {
		var symbol string
		var ticker Ticker
		var to *time.Time
		if err := rows.Scan(&symbol, &ticker.Symbol, &ticker.From, &to); err != nil {
			return nil, fmt.Errorf("failed to scan ticker: %w", err)
		}
		ticker.From = dateOnly(ticker.From)
		if to != nil {
			ticker.To = dateOnly(*to)
		}
		tickers[symbol] = append(tickers[symbol], ticker)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to query tickers: %w", err)
	}
	for _, group := range tickers {
		group[len(group)-1].From = group[0].From
	}
	return tickers, nil
}

// List returns each ticker's span and its bar days from start to end, as
// UTC midnights, counting only the bars inside the ticker's window. A
// ticker with no bars there has no entry.
func (db *BarDays) List(ctx context.Context, provider string, tickers []Ticker, start, end time.Time) (map[string]*SymbolBars, error) {
	symbols := make([]string, len(tickers))
	from := make([]*string, len(tickers))
	to := make([]*string, len(tickers))
	for i, ticker := range tickers {
		symbols[i] = ticker.Symbol
		from[i] = windowDate(ticker.From)
		to[i] = windowDate(ticker.To)
	}
	bars, err := db.spans(ctx, provider, symbols, from, to)
	if err != nil {
		return nil, err
	}
	rows, err := db.Conn.Query(ctx, barDaysQuery, provider, symbols, from, to, dateString(start), dateString(end))
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

// windowDate is a window side as the queries read it: NULL when open.
func windowDate(day time.Time) *string {
	if day.IsZero() {
		return nil
	}
	text := dateString(day)
	return &text
}

func (db *BarDays) spans(ctx context.Context, provider string, symbols []string, from, to []*string) (map[string]*SymbolBars, error) {
	rows, err := db.Conn.Query(ctx, barSpanQuery, provider, symbols, from, to)
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
