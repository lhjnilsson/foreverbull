package auth

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/lhjnilsson/foreverbull/internal/postgres"
	"github.com/lhjnilsson/foreverbull/internal/web"
)

type grantView struct {
	Login     string `json:"login"`
	LoginKind string `json:"login_kind"`
	Level     string `json:"level"`
	Role      string `json:"role"`
}

type grantee struct {
	id        string
	role      string
	login     string
	loginKind string
}

func (h *handlers) putServiceGrant(w http.ResponseWriter, r *http.Request) {
	h.putGrant(w, r, DomainService, r.URL.Query().Get("image"))
}

func (h *handlers) listServiceGrants(w http.ResponseWriter, r *http.Request) {
	h.listGrants(w, r, DomainService, r.URL.Query().Get("image"))
}

func (h *handlers) deleteServiceGrant(w http.ResponseWriter, r *http.Request) {
	h.deleteGrant(w, r, DomainService, r.URL.Query().Get("image"))
}

func (h *handlers) putBacktestGrant(w http.ResponseWriter, r *http.Request) {
	h.putGrant(w, r, DomainBacktest, r.PathValue("name"))
}

func (h *handlers) listBacktestGrants(w http.ResponseWriter, r *http.Request) {
	h.listGrants(w, r, DomainBacktest, r.PathValue("name"))
}

func (h *handlers) deleteBacktestGrant(w http.ResponseWriter, r *http.Request) {
	h.deleteGrant(w, r, DomainBacktest, r.PathValue("name"))
}

func (h *handlers) putStrategyGrant(w http.ResponseWriter, r *http.Request) {
	h.putGrant(w, r, DomainStrategy, r.PathValue("name"))
}

func (h *handlers) listStrategyGrants(w http.ResponseWriter, r *http.Request) {
	h.listGrants(w, r, DomainStrategy, r.PathValue("name"))
}

func (h *handlers) deleteStrategyGrant(w http.ResponseWriter, r *http.Request) {
	h.deleteGrant(w, r, DomainStrategy, r.PathValue("name"))
}

func (h *handlers) putGrant(w http.ResponseWriter, r *http.Request, kind, key string) {
	caller, domainID, ok := h.grantCaller(w, r, kind, key)
	if !ok {
		return
	}
	var body struct {
		Login string `json:"login"`
		Level string `json:"level"`
	}
	if err := web.ReadJSON(r, &body); err != nil {
		web.BadRequest(w)
		return
	}
	level := strings.TrimSpace(body.Level)
	if level != LevelViewer && level != LevelEditor {
		web.BadRequest(w)
		return
	}
	person, err := findGrantee(r.Context(), h.db, body.Login)
	if err != nil {
		WriteHTTP(w, err)
		return
	}
	if person.role == RoleNone || person.role == RolePending {
		h.logf("grant refused owner=%s grantee=%s role=%s", caller.ID, person.id, person.role)
		WriteHTTP(w, ErrNotApproved)
		return
	}
	ownerID, err := domainOwner(r.Context(), h.db, kind, domainID)
	if err != nil {
		WriteHTTP(w, err)
		return
	}
	if person.id == ownerID {
		WriteHTTP(w, ErrOwnerGrant)
		return
	}
	_, err = h.db.Exec(r.Context(), `
		INSERT INTO domain_grant (domain_kind, domain_id, grantee_id, level)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (domain_kind, domain_id, grantee_id)
		DO UPDATE SET level = EXCLUDED.level`, kind, domainID, person.id, level)
	if err != nil {
		web.WriteError(w, err)
		return
	}
	h.logf("grant put owner=%s grantee=%s kind=%s domain=%s level=%s", caller.ID, person.id, kind, domainID, level)
	writeJSON(w, http.StatusOK, grantView{
		Login: person.login, LoginKind: person.loginKind, Level: level, Role: person.role,
	})
}

func (h *handlers) listGrants(w http.ResponseWriter, r *http.Request, kind, key string) {
	_, domainID, ok := h.grantCaller(w, r, kind, key)
	if !ok {
		return
	}
	rows, err := h.db.Query(r.Context(), `
		SELECT COALESCE(p.username, g.github_login, ''),
		       CASE WHEN p.account_id IS NOT NULL THEN 'password' ELSE 'github' END,
		       gr.level, a.role
		FROM domain_grant AS gr
		JOIN account AS a ON a.id = gr.grantee_id
		LEFT JOIN account_password AS p ON p.account_id = a.id
		LEFT JOIN account_github AS g ON g.account_id = a.id
		WHERE gr.domain_kind = $1 AND gr.domain_id = $2
		ORDER BY COALESCE(p.username, g.github_login, '')`, kind, domainID)
	if err != nil {
		web.WriteError(w, err)
		return
	}
	defer rows.Close()
	out := []grantView{}
	for rows.Next() {
		var view grantView
		if err := rows.Scan(&view.Login, &view.LoginKind, &view.Level, &view.Role); err != nil {
			web.WriteError(w, err)
			return
		}
		out = append(out, view)
	}
	if err := rows.Err(); err != nil {
		web.WriteError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *handlers) deleteGrant(w http.ResponseWriter, r *http.Request, kind, key string) {
	caller, domainID, ok := h.grantCaller(w, r, kind, key)
	if !ok {
		return
	}
	person, err := findGrantee(r.Context(), h.db, r.URL.Query().Get("login"))
	if errors.Is(err, ErrAccountNotFound) {
		WriteHTTP(w, ErrGrantNotFound)
		return
	}
	if err != nil {
		WriteHTTP(w, err)
		return
	}
	tag, err := h.db.Exec(r.Context(), `
		DELETE FROM domain_grant
		WHERE domain_kind = $1 AND domain_id = $2 AND grantee_id = $3`, kind, domainID, person.id)
	if err != nil {
		web.WriteError(w, err)
		return
	}
	if tag.RowsAffected() == 0 {
		WriteHTTP(w, ErrGrantNotFound)
		return
	}
	h.logf("grant delete owner=%s grantee=%s kind=%s domain=%s", caller.ID, person.id, kind, domainID)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (h *handlers) grantCaller(w http.ResponseWriter, r *http.Request, kind, key string) (Account, string, bool) {
	caller, ok := FromContext(r.Context())
	if !ok {
		web.Unauthorized(w)
		return Account{}, "", false
	}
	id, err := ResolveGrant(r.Context(), h.db, caller, kind, key)
	if err != nil {
		WriteHTTP(w, err)
		return Account{}, "", false
	}
	return caller, id, true
}

func findGrantee(ctx context.Context, db postgres.Query, login string) (grantee, error) {
	login = strings.TrimSpace(login)
	if login == "" {
		return grantee{}, ErrAccountNotFound
	}
	var person grantee
	err := db.QueryRow(ctx, `
		SELECT a.id, a.role, p.username
		FROM account_password AS p
		JOIN account AS a ON a.id = p.account_id
		WHERE p.username = $1`, login).Scan(&person.id, &person.role, &person.login)
	if err == nil {
		person.loginKind = "password"
		return person, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return grantee{}, err
	}
	rows, err := db.Query(ctx, `
		SELECT a.id, a.role, g.github_login
		FROM account_github AS g
		JOIN account AS a ON a.id = g.account_id
		WHERE g.github_login = $1`, login)
	if err != nil {
		return grantee{}, err
	}
	defer rows.Close()
	found := []grantee{}
	for rows.Next() {
		var item grantee
		if err := rows.Scan(&item.id, &item.role, &item.login); err != nil {
			return grantee{}, err
		}
		item.loginKind = "github"
		found = append(found, item)
	}
	if err := rows.Err(); err != nil {
		return grantee{}, err
	}
	if len(found) == 0 {
		return grantee{}, ErrAccountNotFound
	}
	if len(found) > 1 {
		return grantee{}, ErrLoginAmbiguous
	}
	return found[0], nil
}

func domainOwner(ctx context.Context, db postgres.Query, kind, id string) (string, error) {
	table, _, err := domainTable(kind)
	if err != nil {
		return "", err
	}
	var ownerID string
	err = db.QueryRow(ctx, "SELECT owner_id FROM "+table+" WHERE id = $1", id).Scan(&ownerID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", err
	}
	return ownerID, nil
}
