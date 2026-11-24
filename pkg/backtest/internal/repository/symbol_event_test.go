package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/lhjnilsson/foreverbull/internal/auth"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/repository"
)

// With AUTH_ENABLED the listing and delisting rows are saved and read by
// backtest id, the same key as the ticker resolutions. A row stored by name
// follows the backtest through the move, and the name column is dropped.
func TestSymbolEventsByID(t *testing.T) {
	pool := newBacktestPool(t)
	ctx := context.Background()
	resetBacktests(t, pool)
	seedBacktests(t, pool)
	t.Setenv("AUTH_ENABLED", "false")

	named := repository.Backtest{Conn: pool}
	on := time.Date(2020, 3, 2, 15, 0, 0, 0, time.UTC)
	require.NoError(t, named.SaveSymbolEvents(ctx, "spy", []repository.SymbolEvent{{
		Symbol: "AAPL", Kind: repository.EventListed, On: on, Evidence: "yahoo firstTradeDate 2020-03-02",
	}}))

	admin := insertBacktestAccount(t, pool, auth.RoleAdmin)
	copier := &memIngest{keys: map[string]bool{"spy": true, "qqq": true}}
	require.NoError(t, repository.MigrateBacktestOwner(ctx, pool, admin, copier))
	require.NoError(t, repository.CreateTables(ctx, pool))
	require.NoError(t, repository.MigrateBacktestOwner(ctx, pool, admin, copier))
	t.Setenv("AUTH_ENABLED", "true")

	require.Equal(t, []string{"backtest_id", "symbol", "kind"}, backtestPrimaryKey(t, pool, "backtest_symbol_event"))
	require.False(t, backtestColumn(t, pool, "backtest_symbol_event", "backtest_name"))
	fk := backtestConstraint(t, pool, "backtest_symbol_event", "backtest_symbol_event_backtest_id_fkey")
	require.Contains(t, fk, "FOREIGN KEY (backtest_id) REFERENCES backtest(id)")
	require.Contains(t, fk, "ON DELETE CASCADE")

	owned := repository.Backtest{Conn: pool}
	spy, err := owned.IDByOwnerAndName(ctx, admin, "spy")
	require.NoError(t, err)
	events, err := owned.SymbolEventsByID(ctx, spy)
	require.NoError(t, err)
	require.Len(t, events, 1)
	require.Equal(t, "AAPL", events[0].Symbol)
	require.Equal(t, repository.EventListed, events[0].Kind)
	require.Equal(t, "2020-03-02", events[0].On.Format(time.DateOnly))
	require.Equal(t, "yahoo firstTradeDate 2020-03-02", events[0].Evidence)

	require.Error(t, owned.SaveSymbolEvents(ctx, "spy", nil))
	_, err = owned.SymbolEvents(ctx, "spy")
	require.Error(t, err)
	require.Error(t, owned.SaveSymbolEventsByID(ctx, "", nil))

	require.NoError(t, owned.SaveSymbolEventsByID(ctx, spy, []repository.SymbolEvent{{
		Symbol: "MSFT", Kind: repository.EventDelisted, On: on,
		Evidence: "yahoo search returned no listing and no later ticker",
	}}))
	events, err = owned.SymbolEventsByID(ctx, spy)
	require.NoError(t, err)
	require.Len(t, events, 1)
	require.Equal(t, "MSFT", events[0].Symbol)
	require.Equal(t, repository.EventDelisted, events[0].Kind)

	require.NoError(t, owned.DeleteByID(ctx, spy))
	var left int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM backtest_symbol_event WHERE backtest_id = $1`, spy).Scan(&left))
	require.Zero(t, left)
}
