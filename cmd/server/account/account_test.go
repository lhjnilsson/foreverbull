package account

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/auth"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/internal/test_helper"
	"github.com/stretchr/testify/require"
)

func TestRejectRoleAndShortPasswordDoNotConnect(t *testing.T) {
	t.Setenv(environment.PostgresURL, "postgres://postgres:secret-password@127.0.0.1:1/postgres?sslmode=disable")

	cases := [][]string{
		{"foreverbull", "account", "create", "--username", "reader", "--role", "viewer"},
		{"foreverbull", "account", "create", "--username", "reader", "--role=admin"},
		{"foreverbull", "account", "rotate", "--username", "reader", "--role", "none"},
		{"foreverbull", "account", "rotate", "-role", "pending", "--username", "reader"},
		{"foreverbull", "account", "create", "--username", "reader", "-role=editor"},
	}
	for _, args := range cases {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			err := Run(args, strings.NewReader(" twelve-chars\n"), &stdout, &stderr)
			require.ErrorIs(t, err, ErrRoleChangesAreSQL)
			require.Contains(t, err.Error(), "role changes are SQL")
			require.Empty(t, stdout.String())
			require.NotContains(t, err.Error(), "secret-password")
			require.NotContains(t, stdout.String()+stderr.String(), " twelve-chars")
		})
	}

	var stdout, stderr bytes.Buffer
	err := Run(
		[]string{"foreverbull", "account", "create", "--username", "reader"},
		strings.NewReader("short\n"),
		&stdout,
		&stderr,
	)
	require.ErrorIs(t, err, auth.ErrPasswordLength)
	require.Empty(t, stdout.String())
	require.NotContains(t, err.Error(), "short")
	require.NotContains(t, err.Error(), "secret-password")
}

func TestHelpDoesNotOfferRole(t *testing.T) {
	t.Setenv(environment.PostgresURL, "postgres://postgres:secret-password@127.0.0.1:1/postgres?sslmode=disable")

	var stdout, stderr bytes.Buffer
	err := Run([]string{"foreverbull", "account", "create", "--help"}, strings.NewReader(""), &stdout, &stderr)
	require.NoError(t, err)
	text := stdout.String() + stderr.String()
	require.Contains(t, text, "--username")
	require.NotContains(t, text, "--role")
}

func TestCreateAndRotateUseRealSQL(t *testing.T) {
	test_helper.SetupEnvironment(t, &test_helper.Containers{Postgres: true})

	const (
		username = "reader"
		password = " twelve-chars"
		wrong    = "wrong-password"
		rotated  = "rotated-pass"
	)

	var stdout, stderr bytes.Buffer
	err := Run(
		[]string{"foreverbull", "account", "create", "--username", "  " + username + "  "},
		strings.NewReader(password+"\n"),
		&stdout,
		&stderr,
	)
	require.NoError(t, err)
	require.NotContains(t, stdout.String(), password)
	require.NotContains(t, stderr.String(), password)
	require.NotContains(t, stdout.String(), "argon2")

	id := strings.TrimSpace(stdout.String())
	require.Equal(t, id+"\n", stdout.String())
	require.NotEmpty(t, id)

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, environment.GetPostgresURL())
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	var (
		role string
		hash string
	)
	err = pool.QueryRow(ctx, `
		SELECT a.role, p.password_hash
		FROM account AS a
		JOIN account_password AS p ON p.account_id = a.id
		WHERE a.id = $1`, id).Scan(&role, &hash)
	require.NoError(t, err)
	require.Equal(t, "editor", role)

	ok, err := auth.VerifyPassword(hash, wrong)
	require.NoError(t, err)
	require.False(t, ok)
	ok, err = auth.VerifyPassword(hash, password)
	require.NoError(t, err)
	require.True(t, ok)

	_, err = pool.Exec(ctx, `UPDATE account SET role = 'viewer' WHERE id = $1`, id)
	require.NoError(t, err)

	stdout.Reset()
	stderr.Reset()
	err = Run(
		[]string{"foreverbull", "account", "rotate", "--username", username},
		strings.NewReader(rotated+"\n"),
		&stdout,
		&stderr,
	)
	require.NoError(t, err)
	require.Equal(t, id+"\n", stdout.String())
	require.NotContains(t, stderr.String(), rotated)

	err = pool.QueryRow(ctx, `
		SELECT a.role, p.password_hash
		FROM account AS a
		JOIN account_password AS p ON p.account_id = a.id
		WHERE a.id = $1`, id).Scan(&role, &hash)
	require.NoError(t, err)
	require.Equal(t, "viewer", role)

	ok, err = auth.VerifyPassword(hash, password)
	require.NoError(t, err)
	require.False(t, ok)
	ok, err = auth.VerifyPassword(hash, rotated)
	require.NoError(t, err)
	require.True(t, ok)

	stdout.Reset()
	err = Run(
		[]string{"foreverbull", "account", "create", "--username", username},
		strings.NewReader("other-password\n"),
		&stdout,
		&stderr,
	)
	require.ErrorIs(t, err, auth.ErrUsernameExists)
	require.NotContains(t, err.Error(), "other-password")
	require.Empty(t, stdout.String())

	var hashAfter string
	err = pool.QueryRow(ctx, `SELECT password_hash FROM account_password WHERE username = $1`, username).Scan(&hashAfter)
	require.NoError(t, err)
	require.Equal(t, hash, hashAfter)
}
