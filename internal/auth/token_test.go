package auth

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/lhjnilsson/foreverbull/internal/environment"
)

func TestBearerHonorsExpiresAtAndIgnoresANullOne(t *testing.T) {
	t.Setenv(environment.AuthEnabled, "true")
	pool := newAuthDB(t)
	ctx := context.Background()
	require.NoError(t, Migrate(ctx, pool))

	id, err := CreateEditor(ctx, pool, "token-owner", "twelve-chars")
	require.NoError(t, err)
	minted, err := MintToken(ctx, pool, Account{ID: id, Role: RoleEditor}, "laptop")
	require.NoError(t, err)

	now := time.Now().UTC()
	account, err := AccountFromBearer(ctx, pool, minted.Secret, now)
	require.NoError(t, err)
	require.Equal(t, id, account.ID)

	_, err = pool.Exec(ctx, `UPDATE api_token SET expires_at = $2 WHERE id = $1`, minted.ID, now.Add(-time.Minute))
	require.NoError(t, err)
	_, err = AccountFromBearer(ctx, pool, minted.Secret, now)
	require.ErrorIs(t, err, ErrUnauthorized)

	_, err = pool.Exec(ctx, `UPDATE api_token SET expires_at = $2 WHERE id = $1`, minted.ID, now.Add(time.Hour))
	require.NoError(t, err)
	account, err = AccountFromBearer(ctx, pool, minted.Secret, now)
	require.NoError(t, err)
	require.Equal(t, id, account.ID)
}
