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
	"strconv"
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
}

// githubExchange is replaced in tests. Production calls GitHub.
var githubExchange = exchangeGitHub

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
	sum := sha256.Sum256([]byte(state))
	_, err := h.db.Exec(r.Context(), `
		INSERT INTO oauth_state (state_hash, expires_at) VALUES ($1, $2)`, sum[:], h.now().Add(oauthLifetime))
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
	http.Redirect(w, r, oauthConfig().AuthCodeURL(state), http.StatusFound)
}

func (h *handlers) githubCallback(w http.ResponseWriter, r *http.Request) {
	if !githubConfigured() {
		web.NotFound(w)
		return
	}
	cookie, err := r.Cookie(oauthCookie)
	if err != nil || cookie.Value == "" || cookie.Value != r.URL.Query().Get("state") {
		h.logf("github callback result=unauthorized")
		web.Unauthorized(w)
		return
	}
	sum := sha256.Sum256([]byte(cookie.Value))
	tag, err := h.db.Exec(r.Context(), `
		DELETE FROM oauth_state WHERE state_hash = $1 AND expires_at > $2`, sum[:], h.now())
	if err != nil || tag.RowsAffected() != 1 {
		h.logf("github callback result=unauthorized")
		web.Unauthorized(w)
		return
	}
	identity, err := githubExchange(r.Context(), r.URL.Query().Get("code"))
	if err != nil || identity.Subject == "" || identity.Login == "" {
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
	return &oauth2.Config{
		ClientID:     environment.GetAuthGitHubClientID(),
		ClientSecret: environment.GetAuthGitHubClientSecret(),
		RedirectURL:  environment.GetAuthGitHubRedirectURL(),
		Endpoint:     github.Endpoint,
		Scopes:       []string{"read:user"},
	}
}

func exchangeGitHub(ctx context.Context, code string) (githubIdentity, error) {
	ctx, cancel := context.WithTimeout(ctx, githubTimeout)
	defer cancel()
	tok, err := oauthConfig().Exchange(ctx, code)
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
	return githubIdentity{Subject: strconv.FormatInt(payload.ID, 10), Login: payload.Login}, nil
}
