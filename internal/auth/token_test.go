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
	minted, err := MintToken(ctx, pool, Account{ID: id, Role: RoleEditor}, "laptop", "")
	require.NoError(t, err)

	now := time.Now().UTC()
	account, err := AccountFromBearer(ctx, pool, minted.Secret, now)
	require.NoError(t, err)
	require.Equal(t, id, account.ID)
	require.Equal(t, ScopeWrite, account.Scope)

	_, err = pool.Exec(ctx, `UPDATE api_token SET expires_at = $2 WHERE id = $1`, minted.ID, now.Add(-time.Minute))
	require.NoError(t, err)
	_, err = AccountFromBearer(ctx, pool, minted.Secret, now)
	require.ErrorIs(t, err, ErrUnauthorized)

	_, err = pool.Exec(ctx, `UPDATE api_token SET expires_at = $2 WHERE id = $1`, minted.ID, now.Add(time.Hour))
	require.NoError(t, err)
	account, err = AccountFromBearer(ctx, pool, minted.Secret, now)
	require.NoError(t, err)
	require.Equal(t, id, account.ID)
	require.Equal(t, ScopeWrite, account.Scope)
}

func TestMintStoresScopeAndDefaultsToWrite(t *testing.T) {
	t.Setenv(environment.AuthEnabled, "true")
	pool := newAuthDB(t)
	ctx := context.Background()
	require.NoError(t, Migrate(ctx, pool))

	id, err := CreateEditor(ctx, pool, "scope-owner", "twelve-chars")
	require.NoError(t, err)
	editor := Account{ID: id, Role: RoleEditor}

	_, err = MintToken(ctx, pool, Account{ID: id, Role: RoleViewer}, "nope", ScopeRead)
	require.ErrorIs(t, err, ErrForbidden)
	_, err = MintToken(ctx, pool, editor, "bad", "admin")
	require.ErrorIs(t, err, ErrBadRequest)

	readToken, err := MintToken(ctx, pool, editor, "grafana", ScopeRead)
	require.NoError(t, err)
	require.Equal(t, ScopeRead, readToken.Scope)
	require.NotContains(t, readToken.Secret, "postgres://")

	now := time.Now().UTC()
	account, err := AccountFromBearer(ctx, pool, readToken.Secret, now)
	require.NoError(t, err)
	require.Equal(t, ScopeRead, account.Scope)

	writeToken, err := MintToken(ctx, pool, editor, "laptop", "")
	require.NoError(t, err)
	require.Equal(t, ScopeWrite, writeToken.Scope)
	account, err = AccountFromBearer(ctx, pool, writeToken.Secret, now)
	require.NoError(t, err)
	require.Equal(t, ScopeWrite, account.Scope)

	listed, err := ListTokens(ctx, pool, editor)
	require.NoError(t, err)
	got := map[string]string{}
	for _, token := range listed {
		got[token.Name] = token.Scope
	}
	require.Equal(t, ScopeRead, got["grafana"])
	require.Equal(t, ScopeWrite, got["laptop"])

	_, err = pool.Exec(ctx, `ALTER TABLE api_token DROP CONSTRAINT IF EXISTS api_token_scope_check`)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `ALTER TABLE api_token DROP COLUMN scope`)
	require.NoError(t, err)
	require.NoError(t, Migrate(ctx, pool))

	var scope string
	err = pool.QueryRow(ctx, `SELECT scope FROM api_token WHERE id = $1`, readToken.ID).Scan(&scope)
	require.NoError(t, err)
	require.Equal(t, ScopeWrite, scope)
	account, err = AccountFromBearer(ctx, pool, readToken.Secret, now)
	require.NoError(t, err)
	require.Equal(t, ScopeWrite, account.Scope)

	_, err = pool.Exec(ctx, `UPDATE api_token SET scope = 'admin' WHERE id = $1`, readToken.ID)
	require.Error(t, err)

	kept, err := MintToken(ctx, pool, editor, "dash", ScopeRead)
	require.NoError(t, err)
	require.NoError(t, Migrate(ctx, pool))
	account, err = AccountFromBearer(ctx, pool, kept.Secret, now)
	require.NoError(t, err)
	require.Equal(t, ScopeRead, account.Scope)
}
