package command

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/internal/test_helper"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/repository"
	"github.com/lhjnilsson/foreverbull/pkg/finance"
	"github.com/stretchr/testify/require"
)

// The SQL gap check and the in-memory read have to name the same missing
// sessions. The in-memory read stays the oracle; Gaps is what create uses,
// so a large universe does not load every bar day.
func TestSQLGapsMatchTheInMemoryCheck(t *testing.T) {
	ctx := context.Background()
	test_helper.SetupEnvironment(t, &test_helper.Containers{Postgres: true})
	pool, err := pgxpool.New(ctx, environment.GetPostgresURL())
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	require.NoError(t, repository.Recreate(ctx, pool))
	_, err = pool.Exec(ctx, `
		DROP TABLE IF EXISTS ohlc;
		DROP TABLE IF EXISTS asset;
		CREATE TABLE asset (symbol text primary key, name text);
		CREATE TABLE ohlc (
			symbol text references asset(symbol),
			open numeric NOT NULL, high numeric NOT NULL, low numeric NOT NULL, close numeric NOT NULL,
			volume bigint NOT NULL,
			time timestamp,
			provider text NOT NULL,
			UNIQUE (provider, symbol, time));`)
	require.NoError(t, err)

	// Checked sessions stop at the cap, 2024-03-28. 2024-04-01 is the next
	// session and is not checked. 2024-03-29 is Good Friday, not a session.
	sessions := []time.Time{
		sqlDay(t, "2024-03-25"),
		sqlDay(t, "2024-03-26"),
		sqlDay(t, "2024-03-27"),
		sqlDay(t, "2024-03-28"),
	}
	start, end := sessions[0], sessions[len(sessions)-1]

	_, err = pool.Exec(ctx, `
		INSERT INTO instrument (id, isin, name) VALUES
			('meta', 'US30303M1027', 'Meta Platforms Inc'),
			('x', 'XX0000000001', 'X Corp'),
			('y', 'YY0000000001', 'Y Corp');
		INSERT INTO asset_symbol (isin, symbol, from_date, to_date) VALUES
			('US30303M1027', 'FB', '2024-03-25', '2024-03-26'),
			('US30303M1027', 'META', '2024-03-26', NULL),
			('XX0000000001', 'OLDX', '2010-01-04', '2020-01-02'),
			('XX0000000001', 'NEWX', '2020-01-02', NULL),
			('YY0000000001', 'OLDX', '2021-01-04', NULL);`)
	require.NoError(t, err)

	bars := map[string][]string{
		"OEF":  {"2024-03-25", "2024-03-26", "2024-03-27", "2024-03-28"},
		"GAP":  {"2024-03-22", "2024-03-25", "2024-03-26", "2024-03-28", "2024-04-02"},
		"ENDS": {"2024-03-22", "2024-03-26", "2024-03-27", "2024-04-02"},
		"ZERO": {"2024-03-25", "2024-03-27", "2024-03-28"},
		"HOLI": {"2024-03-25", "2024-03-26", "2024-03-27", "2024-03-28", "2024-04-02"},
		"LATE": {"2024-03-27", "2024-03-28"},
		"META": {"2024-03-25", "2024-03-27", "2024-03-28"},
		"FB":   {"2024-03-26"},
		"NEWX": {"2024-03-25", "2024-03-26", "2024-03-27", "2024-03-28"},
	}
	for symbol, days := range bars {
		sqlStoreBars(t, ctx, pool, symbol, days, 10)
	}
	// A stored 0 is not a bar. 2024-03-26 stays missing, between two prices.
	sqlStoreBars(t, ctx, pool, "ZERO", []string{"2024-03-26"}, 0)

	order := []string{"OEF", "ENDS", "FB", "GAP", "HOLI", "LATE", "OLDX", "ZERO"}
	stored := make([]string, len(order))
	for i, symbol := range order {
		stored[i] = finance.StoredSymbol(symbol)
	}
	barDays := repository.BarDays{Conn: pool}
	tickers, err := barDays.Tickers(ctx, stored)
	require.NoError(t, err)
	units, _ := checkUnits(order, tickers)
	read := make([]repository.Ticker, 0)
	names := make([]string, len(units))
	for i, unit := range units {
		names[i] = unit.name
		read = append(read, unit.tickers...)
	}

	loaded, err := barDays.List(ctx, "yahoo", read, start, end)
	require.NoError(t, err)
	_, oracle := missingSessions(sessions, names, unitBars(units, loaded), nil, true)

	found, err := barDays.Gaps(ctx, "yahoo", unitWindows(units), sessions)
	require.NoError(t, err)
	_, got := missingSessions(sessions, names, barsFromSessionGaps(found), nil, true)
	require.Equal(t, oracle, got)

	bySymbol := make(map[string]symbolGaps, len(got))
	for _, gap := range got {
		bySymbol[gap.symbol] = gap
	}
	require.Equal(t, []int{0, 3}, bySymbol["ENDS"].sessions)
	require.True(t, bySymbol["ENDS"].noneBefore.IsZero())
	require.True(t, bySymbol["ENDS"].noneAfter.IsZero())
	require.Equal(t, []int{2}, bySymbol["GAP"].sessions)
	require.Equal(t, []int{1}, bySymbol["ZERO"].sessions)
	// FB's bar on 2024-03-26 is outside FB's window, so it does not fill META.
	require.Equal(t, []int{1}, bySymbol["META (earlier FB)"].sessions)
	require.True(t, bySymbol["META (earlier FB)"].noneBefore.IsZero())
	require.True(t, bySymbol["META (earlier FB)"].noneAfter.IsZero())
	require.Equal(t, sessions[2], bySymbol["LATE"].noneBefore)
	require.Empty(t, bySymbol["LATE"].sessions)
	require.True(t, bySymbol["OLDX"].noBars)
	require.Equal(t, sessions[0], bySymbol["OLDX"].from)
	require.Equal(t, sessions[3], bySymbol["OLDX"].to)
	_, holiday := bySymbol["HOLI"]
	require.False(t, holiday)
	_, benchmark := bySymbol["OEF"]
	require.False(t, benchmark)
	for _, gap := range got {
		for _, index := range gap.sessions {
			require.NotEqual(t, "2024-03-29", sessions[index].Format(time.DateOnly))
			require.NotEqual(t, "2024-04-01", sessions[index].Format(time.DateOnly))
		}
	}
}

func sqlDay(t *testing.T, value string) time.Time {
	t.Helper()
	day, err := time.Parse(time.DateOnly, value)
	require.NoError(t, err)
	return day
}

func sqlStoreBars(t *testing.T, ctx context.Context, pool *pgxpool.Pool, symbol string, days []string, price int) {
	t.Helper()
	_, err := pool.Exec(ctx, `INSERT INTO asset (symbol, name) VALUES ($1, $1) ON CONFLICT DO NOTHING`, symbol)
	require.NoError(t, err)
	for _, value := range days {
		at := sqlDay(t, value).Add(14*time.Hour + 30*time.Minute)
		_, err = pool.Exec(ctx, `
			INSERT INTO ohlc (symbol, open, high, low, close, volume, time, provider)
			VALUES ($1, $2, $2, $2, $2, 100, $3, 'yahoo')`, symbol, price, at)
		require.NoError(t, err)
	}
}
