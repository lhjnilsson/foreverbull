package auth

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/stretchr/testify/require"
)

func TestStartupRequiresAdminUsername(t *testing.T) {
	t.Setenv(environment.AuthEnabled, "true")
	t.Setenv(environment.AuthAdminUsername, "")
	pool := newAuthDB(t)

	_, err := Startup(context.Background(), pool, func(string, ...any) {})
	require.ErrorContains(t, err, environment.AuthAdminUsername)
}

func TestStartupCreatesAdminFromPasswordFile(t *testing.T) {
	const (
		username = "root"
		password = "admin-password"
	)
	t.Setenv(environment.AuthEnabled, "true")
	t.Setenv(environment.AuthAdminUsername, username)
	path := filepath.Join(t.TempDir(), "password")
	require.NoError(t, os.WriteFile(path, []byte(password+"\n"), 0o600))
	t.Setenv(environment.AuthAdminPasswordFile, path)

	var lines []string
	pool := newAuthDB(t)
	id, err := Startup(context.Background(), pool, func(format string, args ...any) {
		lines = append(lines, fmt.Sprintf(format, args...))
	})
	require.NoError(t, err)
	require.NotEmpty(t, id)

	ctx := context.Background()
	var admins int
	require.NoError(t, pool.QueryRow(ctx, `SELECT COUNT(*) FROM account WHERE role = 'admin'`).Scan(&admins))
	require.Equal(t, 1, admins)
	var accounts int
	require.NoError(t, pool.QueryRow(ctx, `SELECT COUNT(*) FROM account`).Scan(&accounts))
	require.Equal(t, 1, accounts)

	var hash string
	err = pool.QueryRow(ctx, `SELECT password_hash FROM account_password WHERE username = $1`, username).Scan(&hash)
	require.NoError(t, err)
	require.NotEqual(t, password, hash)
	require.True(t, strings.HasPrefix(hash, "$argon2id$"), hash)

	logged := strings.Join(lines, "\n")
	require.NotContains(t, logged, password)
	require.NotContains(t, logged, hash)
}

func TestStartupRefusesEditorPromotion(t *testing.T) {
	t.Setenv(environment.AuthEnabled, "true")
	t.Setenv(environment.AuthAdminUsername, "ada")
	pool := newAuthDB(t)
	ctx := context.Background()
	require.NoError(t, Migrate(ctx, pool))

	var id string
	err := pool.QueryRow(ctx, `INSERT INTO account (role) VALUES ('editor') RETURNING id`).Scan(&id)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `
		INSERT INTO account_password (account_id, username, password_hash)
		VALUES ($1, 'ada', 'x')`, id)
	require.NoError(t, err)

	_, err = Startup(ctx, pool, func(string, ...any) {})
	require.ErrorContains(t, err, environment.AuthAdminUsername)
	require.ErrorContains(t, err, RoleEditor)

	var role, hash string
	err = pool.QueryRow(ctx, `
		SELECT a.role, p.password_hash
		FROM account AS a
		JOIN account_password AS p ON p.account_id = a.id
		WHERE p.username = 'ada'`).Scan(&role, &hash)
	require.NoError(t, err)
	require.Equal(t, RoleEditor, role)
	require.Equal(t, "x", hash)
}

func TestPendingAndNoneCannotWrite(t *testing.T) {
	t.Setenv(environment.AuthEnabled, "true")
	pool := newAuthDB(t)
	ctx := context.Background()
	require.NoError(t, Migrate(ctx, pool))

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/services", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	RegisterHTTP(mux, pool)
	handler := Protect(pool)(mux)

	for _, role := range []string{RolePending, RoleNone} {
		t.Run(role, func(t *testing.T) {
			var id string
			err := pool.QueryRow(ctx, `INSERT INTO account (role) VALUES ($1) RETURNING id`, role).Scan(&id)
			require.NoError(t, err)
			cookie, err := OpenSession(ctx, pool, id, time.Now())
			require.NoError(t, err)

			require.Equal(t, http.StatusForbidden, authedStatus(handler, http.MethodPost, "/api/services", `{"image":"algo:local"}`, cookie))
			require.Equal(t, http.StatusForbidden, authedStatus(handler, http.MethodPost, "/api/auth/tokens", `{"name":"cli"}`, cookie))

			rec := authed(handler, http.MethodGet, "/api/auth/me", "", cookie)
			require.Equal(t, http.StatusOK, rec.Code)
			require.Contains(t, rec.Body.String(), `"role":"`+role+`"`)
		})
	}
}

func authed(handler http.Handler, method, path, body, cookie string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Cookie", "fb_session="+cookie)
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func authedStatus(handler http.Handler, method, path, body, cookie string) int {
	return authed(handler, method, path, body, cookie).Code
}
