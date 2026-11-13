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
	// First and Last are the symbol's first and last positive bar over all
	// stored time, as UTC dates, so a window that starts or ends inside the
	// span still sees the sessions missing at its edges.
	First, Last time.Time
	// Days are the bar days inside the window, oldest first. List fills
	// them. When Absent is non-nil, Days is unused.
	Days []time.Time
	// Absent, when non-nil, is the sessions inside [First, Last] with no
	// positive bar, oldest first. An empty slice means every such session
	// has a bar. Nil means Days is the bar list instead.
	Absent []time.Time
}

// UnitWindow is one ticker window of a named unit. Several windows share a
// unit when an instrument's tickers are read together. A zero From or To
// leaves that side open.
type UnitWindow struct {
	Unit     string
	Symbol   string
	From, To time.Time
}

// SessionGap is one unit's span and the checked sessions inside it that
// have no positive bar. A unit with no positive bar is absent.
type SessionGap struct {
	First, Last time.Time
	Missing     []time.Time
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

// gapQuery is the missing-session check in SQL. Sessions are left-joined to
// the positive bars inside each unit's ticker windows, so a large universe
// does not load every bar day. The UTC date matches List, which reads
// dateOnly(time.UTC()) in Go. A unit with no positive bar is not returned.
const gapQuery = `
WITH w AS (
	SELECT unit, symbol,
	       lo::timestamp AT TIME ZONE 'UTC' AS lo,
	       hi::timestamp AT TIME ZONE 'UTC' AS hi
	  FROM unnest($2::text[], $3::text[], $4::text[]::date[], $5::text[]::date[])
	    AS t(unit, symbol, lo, hi)),
positive AS (
	SELECT DISTINCT w.unit,
	       ((o.time AT TIME ZONE 'UTC') AT TIME ZONE 'UTC')::date AS day
	  FROM w
	  JOIN ohlc o ON o.symbol = w.symbol
	 WHERE o.provider = $1
	   AND (w.lo IS NULL OR o.time >= w.lo)
	   AND (w.hi IS NULL OR o.time < w.hi)
	   AND o.open > 0 AND o.high > 0 AND o.low > 0 AND o.close > 0),
span AS (
	SELECT unit, min(day) AS first, max(day) AS last
	  FROM positive
	 GROUP BY unit),
sessions AS (
	SELECT d::date AS day FROM unnest($6::text[]::date[]) AS d)
SELECT unit, first, last, missing FROM (
	SELECT unit, first, last, NULL::date AS missing FROM span
	UNION ALL
	SELECT s.unit, s.first, s.last, sess.day
	  FROM span s
	  JOIN sessions sess ON sess.day >= s.first AND sess.day <= s.last
	  LEFT JOIN positive p ON p.unit = s.unit AND p.day = sess.day
	 WHERE p.day IS NULL
) gaps
ORDER BY unit, missing NULLS FIRST`

// Gaps returns each unit's span and the sessions inside that span with no
// positive bar in any of its ticker windows. sessions are the dates the
// check looks at, already capped at the last completed session and without
// holidays. A unit with no positive bar has no entry.
func (db *BarDays) Gaps(ctx context.Context, provider string, windows []UnitWindow, sessions []time.Time) (map[string]*SessionGap, error) {
	if len(windows) == 0 {
		return map[string]*SessionGap{}, nil
	}
	units := make([]string, len(windows))
	symbols := make([]string, len(windows))
	from := make([]*string, len(windows))
	to := make([]*string, len(windows))
	for i, window := range windows {
		units[i] = window.Unit
		symbols[i] = window.Symbol
		from[i] = windowDate(window.From)
		to[i] = windowDate(window.To)
	}
	days := make([]string, len(sessions))
	for i, session := range sessions {
		days[i] = dateString(session)
	}
	rows, err := db.Conn.Query(ctx, gapQuery, provider, units, symbols, from, to, days)
	if err != nil {
		return nil, fmt.Errorf("failed to query bar gaps: %w", err)
	}
	defer rows.Close()
	gaps := make(map[string]*SessionGap)
	for rows.Next() {
		var unit string
		var first, last time.Time
		var missing *time.Time
		if err := rows.Scan(&unit, &first, &last, &missing); err != nil {
			return nil, fmt.Errorf("failed to scan bar gap: %w", err)
		}
		entry := gaps[unit]
		if entry == nil {
			entry = &SessionGap{First: dateOnly(first.UTC()), Last: dateOnly(last.UTC()), Missing: []time.Time{}}
			gaps[unit] = entry
		}
		if missing == nil {
			continue
		}
		entry.Missing = append(entry.Missing, dateOnly(missing.UTC()))
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to query bar gaps: %w", err)
	}
	return gaps, nil
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
