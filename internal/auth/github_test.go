package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"

	"github.com/lhjnilsson/foreverbull/internal/environment"
)

func TestGitHubLoginUsesPKCEAndRejectsAForeignOrg(t *testing.T) {
	t.Setenv(environment.AuthEnabled, "true")
	t.Setenv(environment.AuthGitHubClientID, "client")
	t.Setenv(environment.AuthGitHubClientSecret, "secret")
	t.Setenv(environment.AuthGitHubRedirectURL, "http://localhost:8080/api/auth/github/callback")
	t.Setenv(environment.AuthGitHubAllowedOrgs, "foreverbull")

	pool := newAuthDB(t)
	ctx := context.Background()
	require.NoError(t, Migrate(ctx, pool))

	previousExchange := githubExchange
	previousMember := githubMember
	t.Cleanup(func() {
		githubExchange = previousExchange
		githubMember = previousMember
	})
	var gotVerifier string
	githubExchange = func(_ context.Context, code, verifier string) (githubIdentity, error) {
		require.Equal(t, "code-1", code)
		gotVerifier = verifier
		return githubIdentity{Subject: "42", Login: "ada", token: &oauth2.Token{AccessToken: "not-logged"}}, nil
	}
	githubMember = func(context.Context, *oauth2.Token, string) bool { return false }

	mux := http.NewServeMux()
	registerHTTP(mux, pool, func(string, ...any) {})
	start := httptest.NewRecorder()
	mux.ServeHTTP(start, httptest.NewRequest(http.MethodGet, "/api/auth/github/start", nil))
	require.Equal(t, http.StatusFound, start.Code)
	location := start.Header().Get("Location")
	require.Contains(t, location, "code_challenge=")
	require.Contains(t, location, "code_challenge_method=S256")
	require.Contains(t, location, "read%3Aorg")

	state := oauthStateCookie(t, start)
	callback := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/auth/github/callback?code=code-1&state="+url.QueryEscape(state), nil)
	req.AddCookie(&http.Cookie{Name: oauthCookie, Value: state})
	mux.ServeHTTP(callback, req)
	require.Equal(t, http.StatusUnauthorized, callback.Code)
	require.NotEmpty(t, gotVerifier)
	require.NotContains(t, location, gotVerifier)
	require.NotContains(t, callback.Body.String(), gotVerifier)
	require.NotContains(t, callback.Body.String(), "not-logged")

	var accounts int
	require.NoError(t, pool.QueryRow(ctx, `SELECT COUNT(*) FROM account`).Scan(&accounts))
	require.Zero(t, accounts)

	githubMember = func(_ context.Context, _ *oauth2.Token, org string) bool {
		return org == "foreverbull"
	}
	start = httptest.NewRecorder()
	mux.ServeHTTP(start, httptest.NewRequest(http.MethodGet, "/api/auth/github/start", nil))
	state = oauthStateCookie(t, start)
	callback = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/auth/github/callback?code=code-1&state="+url.QueryEscape(state), nil)
	req.AddCookie(&http.Cookie{Name: oauthCookie, Value: state})
	mux.ServeHTTP(callback, req)
	require.Equal(t, http.StatusFound, callback.Code)

	var role string
	err := pool.QueryRow(ctx, `
		SELECT a.role
		FROM account_github AS g
		JOIN account AS a ON a.id = g.account_id
		WHERE g.github_subject = '42'`).Scan(&role)
	require.NoError(t, err)
	require.Equal(t, RolePending, role)
}

func oauthStateCookie(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	for _, cookie := range rec.Result().Cookies() {
		if cookie.Name == oauthCookie {
			require.NotEmpty(t, cookie.Value)
			return cookie.Value
		}
	}
	t.Fatal("oauth state cookie was not set")
	return ""
}
