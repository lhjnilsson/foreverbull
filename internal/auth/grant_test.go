package auth

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/lhjnilsson/foreverbull/internal/environment"
)

func TestFindGranteeDoesNotShadowAGitHubLogin(t *testing.T) {
	t.Setenv(environment.AuthEnabled, "true")
	pool := newAuthDB(t)
	ctx := context.Background()
	require.NoError(t, Migrate(ctx, pool))

	passwordID, err := CreateEditor(ctx, pool, "bob", "twelve-chars")
	require.NoError(t, err)
	adaID, err := CreateEditor(ctx, pool, "ada", "twelve-chars")
	require.NoError(t, err)

	var githubID string
	err = pool.QueryRow(ctx, `INSERT INTO account (role) VALUES ('editor') RETURNING id`).Scan(&githubID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `
		INSERT INTO account_github (account_id, github_subject, github_login)
		VALUES ($1, '99', 'bob')`, githubID)
	require.NoError(t, err)
	var caraID string
	err = pool.QueryRow(ctx, `INSERT INTO account (role) VALUES ('editor') RETURNING id`).Scan(&caraID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `
		INSERT INTO account_github (account_id, github_subject, github_login)
		VALUES ($1, '7', 'cara')`, caraID)
	require.NoError(t, err)

	_, err = findGrantee(ctx, pool, "bob")
	require.ErrorIs(t, err, ErrLoginAmbiguous)

	person, err := findGrantee(ctx, pool, "user:bob")
	require.NoError(t, err)
	require.Equal(t, passwordID, person.id)
	require.Equal(t, "user:bob", person.login)
	require.Equal(t, "password", person.loginKind)

	person, err = findGrantee(ctx, pool, "github:99")
	require.NoError(t, err)
	require.Equal(t, githubID, person.id)
	require.Equal(t, "github:99", person.login)
	require.NotEqual(t, passwordID, person.id)

	_, err = findGrantee(ctx, pool, "github:bob")
	require.ErrorIs(t, err, ErrAccountNotFound)

	person, err = findGrantee(ctx, pool, passwordID)
	require.NoError(t, err)
	require.Equal(t, passwordID, person.id)

	person, err = findGrantee(ctx, pool, "ada")
	require.NoError(t, err)
	require.Equal(t, adaID, person.id)
	require.Equal(t, "user:ada", person.login)

	person, err = findGrantee(ctx, pool, "cara")
	require.NoError(t, err)
	require.Equal(t, caraID, person.id)
	require.Equal(t, "github:7", person.login)
}
