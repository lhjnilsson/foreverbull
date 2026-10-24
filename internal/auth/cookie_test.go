package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/lhjnilsson/foreverbull/internal/environment"
)

func TestSessionCookieUsesHostPrefixWhenSecure(t *testing.T) {
	t.Setenv(environment.AuthEnabled, "true")
	t.Setenv(environment.AuthCookieSecure, "true")
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	SetSessionCookie(rec, req, "abc.def", time.Unix(1_700_000_000, 0))
	cookie := rec.Result().Cookies()[0]
	require.Equal(t, hostSessionCookie, cookie.Name)
	require.True(t, cookie.Secure)
	require.Equal(t, "/", cookie.Path)
	require.Empty(t, cookie.Domain)
	require.True(t, cookie.HttpOnly)

	read := httptest.NewRequest(http.MethodGet, "/", nil)
	read.AddCookie(cookie)
	found, err := read.Cookie(sessionCookieName(read))
	require.NoError(t, err)
	require.Equal(t, "abc.def", found.Value)
}

func TestSessionCookieStaysPlainWithoutTLS(t *testing.T) {
	t.Setenv(environment.AuthEnabled, "true")
	t.Setenv(environment.AuthCookieSecure, "false")
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	SetSessionCookie(rec, req, "abc.def", time.Unix(1_700_000_000, 0))
	cookie := rec.Result().Cookies()[0]
	require.Equal(t, sessionCookie, cookie.Name)
	require.False(t, cookie.Secure)
	require.NotEqual(t, hostSessionCookie, cookie.Name)
}
