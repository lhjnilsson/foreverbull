package auth

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/lhjnilsson/foreverbull/internal/environment"
)

func TestSweepExpiredDropsOnlyPastRows(t *testing.T) {
	t.Setenv(environment.AuthEnabled, "true")
	pool := newAuthDB(t)
	ctx := context.Background()
	require.NoError(t, Migrate(ctx, pool))

	id, err := CreateEditor(ctx, pool, "sweep-owner", "twelve-chars")
	require.NoError(t, err)
	now := time.Now().UTC()
	_, err = pool.Exec(ctx, `
		INSERT INTO oauth_state (state_hash, expires_at) VALUES ($1, $2)`, []byte("old-state"), now.Add(-time.Minute))
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `
		INSERT INTO oauth_state (state_hash, expires_at) VALUES ($1, $2)`, []byte("new-state"), now.Add(time.Hour))
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `
		INSERT INTO browser_session (id, account_id, secret_hash, expires_at)
		VALUES ('old-session', $1, $2, $3)`, id, []byte("old"), now.Add(-time.Minute))
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `
		INSERT INTO browser_session (id, account_id, secret_hash, expires_at)
		VALUES ('new-session', $1, $2, $3)`, id, []byte("new"), now.Add(time.Hour))
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `
		INSERT INTO browser_session (id, account_id, secret_hash, expires_at, revoked_at)
		VALUES ('revoked-session', $1, $2, $3, $4)`, id, []byte("rev"), now.Add(time.Hour), now.Add(-time.Minute))
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `
		INSERT INTO browser_session (id, account_id, secret_hash, expires_at, last_seen_at)
		VALUES ('idle-session', $1, $2, $3, $4)`, id, []byte("idle"), now.Add(time.Hour), now.Add(-8*24*time.Hour))
	require.NoError(t, err)

	sweepExpired(ctx, pool, now, nil)

	var states, sessions int
	err = pool.QueryRow(ctx, `SELECT COUNT(*) FROM oauth_state`).Scan(&states)
	require.NoError(t, err)
	require.Equal(t, 1, states)
	err = pool.QueryRow(ctx, `SELECT COUNT(*) FROM browser_session WHERE id = 'new-session'`).Scan(&sessions)
	require.NoError(t, err)
	require.Equal(t, 1, sessions)
	err = pool.QueryRow(ctx, `SELECT COUNT(*) FROM browser_session WHERE id = 'old-session'`).Scan(&sessions)
	require.NoError(t, err)
	require.Equal(t, 0, sessions)
	err = pool.QueryRow(ctx, `SELECT COUNT(*) FROM browser_session WHERE id IN ('revoked-session', 'idle-session')`).Scan(&sessions)
	require.NoError(t, err)
	require.Equal(t, 0, sessions)
}

func TestSweepLogsAFailedDelete(t *testing.T) {
	t.Setenv(environment.AuthEnabled, "true")
	pool := newAuthDB(t)
	ctx := context.Background()
	require.NoError(t, Migrate(ctx, pool))
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	var logged string
	sweepExpired(canceled, pool, time.Now(), func(format string, args ...any) {
		logged = fmt.Sprintf(format, args...)
	})
	require.Contains(t, logged, "sweep")
	require.NotContains(t, logged, "password")
	require.NotContains(t, logged, "postgres://")
}
