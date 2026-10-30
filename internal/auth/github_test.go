package auth

import (
	"context"
	"errors"
	"fmt"
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
	githubMember = func(context.Context, *oauth2.Token, string) (bool, error) { return false, nil }

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

	githubMember = func(_ context.Context, _ *oauth2.Token, org string) (bool, error) {
		return org == "foreverbull", nil
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

func TestGitHubLoginSaysTheOrgCheckFailedWhenGitHubCannotAnswer(t *testing.T) {
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
	githubExchange = func(context.Context, string, string) (githubIdentity, error) {
		return githubIdentity{Subject: "42", Login: "ada", token: &oauth2.Token{AccessToken: "not-logged"}}, nil
	}
	githubMember = func(context.Context, *oauth2.Token, string) (bool, error) {
		return false, errors.New("github org foreverbull membership status 502")
	}

	var logged []string
	mux := http.NewServeMux()
	registerHTTP(mux, pool, func(format string, args ...any) { logged = append(logged, fmt.Sprintf(format, args...)) })
	start := httptest.NewRecorder()
	mux.ServeHTTP(start, httptest.NewRequest(http.MethodGet, "/api/auth/github/start", nil))
	state := oauthStateCookie(t, start)
	callback := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/auth/github/callback?code=code-1&state="+url.QueryEscape(state), nil)
	req.AddCookie(&http.Cookie{Name: oauthCookie, Value: state})
	mux.ServeHTTP(callback, req)

	require.Equal(t, http.StatusBadGateway, callback.Code)
	require.JSONEq(t, `{"error":"github organization membership could not be checked"}`, callback.Body.String())
	require.Contains(t, logged, "github callback result=org_check_failed login=ada: github org foreverbull membership status 502")
	for _, line := range logged {
		require.NotContains(t, line, "not-logged")
	}
	for _, cookie := range callback.Result().Cookies() {
		require.NotEqual(t, sessionCookie, cookie.Name)
		require.NotEqual(t, hostSessionCookie, cookie.Name)
	}
	var accounts int
	require.NoError(t, pool.QueryRow(ctx, `SELECT COUNT(*) FROM account`).Scan(&accounts))
	require.Zero(t, accounts)
}

func TestGitHubOrgAllowedPrefersAMembershipOverAnotherOrgsError(t *testing.T) {
	t.Setenv(environment.AuthGitHubAllowedOrgs, "down, foreverbull")
	previousMember := githubMember
	t.Cleanup(func() { githubMember = previousMember })
	githubMember = func(_ context.Context, _ *oauth2.Token, org string) (bool, error) {
		if org == "down" {
			return false, errors.New("github org down membership status 503")
		}
		return true, nil
	}
	allowed, err := githubOrgAllowed(context.Background(), githubIdentity{token: &oauth2.Token{}})
	require.NoError(t, err)
	require.True(t, allowed)

	githubMember = func(_ context.Context, _ *oauth2.Token, org string) (bool, error) {
		if org == "down" {
			return false, errors.New("github org down membership status 503")
		}
		return false, nil
	}
	allowed, err = githubOrgAllowed(context.Background(), githubIdentity{token: &oauth2.Token{}})
	require.EqualError(t, err, "github org down membership status 503")
	require.False(t, allowed)
}

func TestGitHubOrgMemberSeparatesAnAnswerFromAFailure(t *testing.T) {
	cases := []struct {
		name    string
		status  int
		body    string
		member  bool
		wantErr string
	}{
		{name: "active member", status: http.StatusOK, body: `{"state":"active"}`, member: true},
		{name: "pending member", status: http.StatusOK, body: `{"state":"pending"}`},
		{name: "not a member", status: http.StatusNotFound, body: `{"message":"Not Found"}`},
		{name: "server error", status: http.StatusInternalServerError, body: `oops`, wantErr: "github org foreverbull membership status 500"},
		{name: "bad gateway", status: http.StatusBadGateway, body: ``, wantErr: "github org foreverbull membership status 502"},
		{name: "forbidden", status: http.StatusForbidden, body: `{}`, wantErr: "github org foreverbull membership status 403"},
		{name: "unreadable body", status: http.StatusOK, body: `not json`, wantErr: "github org foreverbull membership body is not JSON: invalid character 'o' in literal null (expecting 'u')"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var gotPath, gotAuth string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotPath = r.URL.Path
				gotAuth = r.Header.Get("Authorization")
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			t.Cleanup(srv.Close)
			previous := githubMembershipAPI
			t.Cleanup(func() { githubMembershipAPI = previous })
			githubMembershipAPI = srv.URL + "/user/memberships/orgs/"

			member, err := githubOrgMember(context.Background(), &oauth2.Token{AccessToken: "tok"}, "foreverbull")
			require.Equal(t, "/user/memberships/orgs/foreverbull", gotPath)
			require.Equal(t, "Bearer tok", gotAuth)
			if tc.wantErr == "" {
				require.NoError(t, err)
			} else {
				require.EqualError(t, err, tc.wantErr)
			}
			require.Equal(t, tc.member, member)
		})
	}
}

func TestGitHubOrgMemberFailsWhenGitHubIsUnreachable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	srv.Close()
	previous := githubMembershipAPI
	t.Cleanup(func() { githubMembershipAPI = previous })
	githubMembershipAPI = srv.URL + "/user/memberships/orgs/"

	member, err := githubOrgMember(context.Background(), &oauth2.Token{AccessToken: "tok"}, "foreverbull")
	require.ErrorContains(t, err, "github org foreverbull membership: ")
	require.False(t, member)
}
