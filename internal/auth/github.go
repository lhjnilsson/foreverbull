package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"

	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/internal/web"
)

const (
	oauthCookie    = "fb_oauth_state"
	oauthLifetime  = 10 * time.Minute
	githubUserAPI  = "https://api.github.com/user"
	githubTimeout  = 10 * time.Second
	oauthStateSize = 32
)

type githubIdentity struct {
	Subject string
	Login   string
	token   *oauth2.Token
}

// githubExchange and githubMember are replaced in tests. Production calls GitHub.
var (
	githubExchange = exchangeGitHub
	githubMember   = githubOrgMember
)

func (h *handlers) githubStart(w http.ResponseWriter, r *http.Request) {
	if !githubConfigured() {
		web.NotFound(w)
		return
	}
	raw := make([]byte, oauthStateSize)
	if _, err := rand.Read(raw); err != nil {
		web.WriteError(w, err)
		return
	}
	state := base64.RawURLEncoding.EncodeToString(raw)
	verifier := oauth2.GenerateVerifier()
	sum := sha256.Sum256([]byte(state))
	_, err := h.db.Exec(r.Context(), `
		INSERT INTO oauth_state (state_hash, expires_at, verifier) VALUES ($1, $2, $3)`,
		sum[:], h.now().Add(oauthLifetime), verifier)
	if err != nil {
		web.WriteError(w, err)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     oauthCookie,
		Value:    state,
		Path:     "/api/auth/github",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   cookieSecure(r),
		MaxAge:   int(oauthLifetime / time.Second),
	})
	http.Redirect(w, r, oauthConfig().AuthCodeURL(state, oauth2.S256ChallengeOption(verifier)), http.StatusFound)
}

func (h *handlers) githubCallback(w http.ResponseWriter, r *http.Request) {
	if !githubConfigured() {
		web.NotFound(w)
		return
	}
	cookie, err := r.Cookie(oauthCookie)
	if cookie != nil {
		clearOAuthCookie(w, r)
	}
	if err != nil || cookie.Value == "" || cookie.Value != r.URL.Query().Get("state") {
		h.logf("github callback result=unauthorized")
		web.Unauthorized(w)
		return
	}
	sum := sha256.Sum256([]byte(cookie.Value))
	var verifier string
	err = h.db.QueryRow(r.Context(), `
		DELETE FROM oauth_state
		WHERE state_hash = $1 AND expires_at > $2
		RETURNING verifier`, sum[:], h.now()).Scan(&verifier)
	if err != nil || verifier == "" {
		h.logf("github callback result=unauthorized")
		web.Unauthorized(w)
		return
	}
	identity, err := githubExchange(r.Context(), r.URL.Query().Get("code"), verifier)
	if err != nil || identity.Subject == "" || identity.Login == "" || !githubOrgAllowed(r.Context(), identity) {
		h.logf("github callback result=unauthorized")
		web.Unauthorized(w)
		return
	}
	account, err := h.upsertGitHub(r.Context(), identity)
	if err != nil {
		web.WriteError(w, err)
		return
	}
	value, err := OpenSession(r.Context(), h.db, account.ID, h.now())
	if err != nil {
		web.WriteError(w, err)
		return
	}
	SetSessionCookie(w, r, value, h.now())
	h.logf("github callback result=ok account=%s", account.ID)
	http.Redirect(w, r, "/", http.StatusFound)
}

func clearOAuthCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     oauthCookie,
		Value:    "",
		Path:     "/api/auth/github",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   cookieSecure(r),
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
	})
}

func (h *handlers) upsertGitHub(ctx context.Context, identity githubIdentity) (Account, error) {
	var account Account
	err := h.db.QueryRow(ctx, `
		SELECT a.id, a.role
		FROM account_github AS g
		JOIN account AS a ON a.id = g.account_id
		WHERE g.github_subject = $1`, identity.Subject).Scan(&account.ID, &account.Role)
	if err == nil {
		_, _ = h.db.Exec(ctx, `UPDATE account_github SET github_login = $2 WHERE account_id = $1`, account.ID, identity.Login)
		return account, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return Account{}, fmt.Errorf("lookup github account: %w", err)
	}
	starter, ok := h.db.(txBeginner)
	if !ok || starter == nil {
		return Account{}, errors.New("github account database cannot begin")
	}
	tx, err := starter.Begin(ctx)
	if err != nil {
		return Account{}, fmt.Errorf("begin github account: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	err = tx.QueryRow(ctx, `INSERT INTO account (role) VALUES ('pending') RETURNING id`).Scan(&account.ID)
	if err != nil {
		return Account{}, fmt.Errorf("insert github account: %w", err)
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO account_github (account_id, github_subject, github_login)
		VALUES ($1, $2, $3)`, account.ID, identity.Subject, identity.Login)
	if err != nil {
		return Account{}, fmt.Errorf("insert github identity: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return Account{}, fmt.Errorf("commit github account: %w", err)
	}
	account.Role = RolePending
	return account, nil
}

type txBeginner interface {
	Begin(ctx context.Context) (pgx.Tx, error)
}

func oauthConfig() *oauth2.Config {
	scopes := []string{"read:user"}
	if strings.TrimSpace(environment.GetAuthGitHubAllowedOrgs()) != "" {
		scopes = append(scopes, "read:org")
	}
	return &oauth2.Config{
		ClientID:     environment.GetAuthGitHubClientID(),
		ClientSecret: environment.GetAuthGitHubClientSecret(),
		RedirectURL:  environment.GetAuthGitHubRedirectURL(),
		Endpoint:     github.Endpoint,
		Scopes:       scopes,
	}
}

func githubOrgAllowed(ctx context.Context, identity githubIdentity) bool {
	orgs := allowedOrgs()
	if len(orgs) == 0 {
		return true
	}
	for _, org := range orgs {
		if githubMember(ctx, identity.token, org) {
			return true
		}
	}
	return false
}

func allowedOrgs() []string {
	raw := strings.TrimSpace(environment.GetAuthGitHubAllowedOrgs())
	if raw == "" {
		return nil
	}
	var orgs []string
	for _, field := range strings.Split(raw, ",") {
		field = strings.TrimSpace(field)
		if field != "" {
			orgs = append(orgs, field)
		}
	}
	return orgs
}

func githubOrgMember(ctx context.Context, tok *oauth2.Token, org string) bool {
	if tok == nil || org == "" || strings.ContainsAny(org, "/?#") {
		return false
	}
	ctx, cancel := context.WithTimeout(ctx, githubTimeout)
	defer cancel()
	endpoint := "https://api.github.com/user/memberships/orgs/" + url.PathEscape(org)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return false
	}
	resp, err := oauthConfig().Client(ctx, tok).Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil || resp.StatusCode != http.StatusOK {
		return false
	}
	var payload struct {
		State string `json:"state"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return false
	}
	return payload.State == "active"
}

func exchangeGitHub(ctx context.Context, code, verifier string) (githubIdentity, error) {
	ctx, cancel := context.WithTimeout(ctx, githubTimeout)
	defer cancel()
	if verifier == "" {
		return githubIdentity{}, errors.New("github verifier is empty")
	}
	tok, err := oauthConfig().Exchange(ctx, code, oauth2.VerifierOption(verifier))
	if err != nil {
		return githubIdentity{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, githubUserAPI, nil)
	if err != nil {
		return githubIdentity{}, err
	}
	resp, err := oauthConfig().Client(ctx, tok).Do(req)
	if err != nil {
		return githubIdentity{}, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return githubIdentity{}, err
	}
	if resp.StatusCode != http.StatusOK {
		return githubIdentity{}, fmt.Errorf("github user status %d", resp.StatusCode)
	}
	var payload struct {
		ID    int64  `json:"id"`
		Login string `json:"login"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return githubIdentity{}, err
	}
	if payload.ID == 0 || payload.Login == "" {
		return githubIdentity{}, errors.New("github user is incomplete")
	}
	return githubIdentity{
		Subject: strconv.FormatInt(payload.ID, 10),
		Login:   payload.Login,
		token:   tok,
	}, nil
}
