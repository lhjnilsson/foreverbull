package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/lhjnilsson/foreverbull/internal/postgres"
)

type rowHit struct {
	id         string
	ownerID    string
	ownerLogin string
	grantLevel string
}

// Resolve returns the domain id the caller may use for action.
// An empty account id does not query. A hidden row is not found.
func Resolve(ctx context.Context, db postgres.Query, caller Account, kind, key, ownerLogin, action string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if caller.ID == "" {
		return "", errors.New("account id is empty")
	}
	key = strings.TrimSpace(key)
	ownerLogin = strings.TrimSpace(ownerLogin)
	if key == "" {
		return "", ErrNotFound
	}
	hits, err := loadHits(ctx, db, caller, kind, key)
	if err != nil {
		return "", err
	}
	chosen, err := chooseHit(caller, hits, ownerLogin)
	if err != nil {
		return "", err
	}
	owner := chosen.ownerID == caller.ID
	visible := caller.Role == RoleAdmin || owner && caller.Role == RoleEditor || chosen.grantLevel != ""
	if caller.Role == RoleViewer {
		visible = chosen.grantLevel != ""
		owner = false
	}
	if err := Allow(caller, action, chosen.grantLevel, owner, visible); err != nil {
		return "", err
	}
	return chosen.id, nil
}

// Can reports whether caller may perform action on a domain id that is already known.
func Can(ctx context.Context, db postgres.Query, caller Account, kind, id, action string) error {
	if caller.ID == "" {
		return errors.New("account id is empty")
	}
	if id == "" {
		return ErrNotFound
	}
	hit, err := loadHitByID(ctx, db, caller, kind, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	owner := hit.ownerID == caller.ID
	visible := caller.Role == RoleAdmin || (owner && caller.Role == RoleEditor) || hit.grantLevel != ""
	if caller.Role == RoleViewer {
		visible = hit.grantLevel != ""
		owner = false
	}
	return Allow(caller, action, hit.grantLevel, owner, visible)
}

func chooseHit(caller Account, hits []rowHit, ownerLogin string) (rowHit, error) {
	if ownerLogin != "" {
		matched := make([]rowHit, 0, 1)
		for _, hit := range hits {
			if hit.ownerLogin == ownerLogin && hitVisible(caller, hit) {
				matched = append(matched, hit)
			}
		}
		if len(matched) != 1 {
			return rowHit{}, ErrNotFound
		}
		return matched[0], nil
	}
	if caller.Role == RoleAdmin {
		for _, hit := range hits {
			if hit.ownerID == caller.ID {
				return hit, nil
			}
		}
		if len(hits) == 1 {
			return hits[0], nil
		}
		if len(hits) > 1 {
			return rowHit{}, ErrNameAmbiguous
		}
		return rowHit{}, ErrNotFound
	}
	if caller.Role == RoleEditor {
		for _, hit := range hits {
			if hit.ownerID == caller.ID {
				return hit, nil
			}
		}
	}
	grants := make([]rowHit, 0, len(hits))
	for _, hit := range hits {
		if hit.grantLevel != "" && hit.ownerID != caller.ID {
			grants = append(grants, hit)
		}
	}
	if len(grants) == 1 {
		return grants[0], nil
	}
	if len(grants) > 1 {
		return rowHit{}, ErrNameAmbiguous
	}
	return rowHit{}, ErrNotFound
}

func hitVisible(caller Account, hit rowHit) bool {
	if caller.Role == RoleAdmin {
		return true
	}
	if caller.Role == RoleEditor && hit.ownerID == caller.ID {
		return true
	}
	return hit.grantLevel != ""
}

func loadHits(ctx context.Context, db postgres.Query, caller Account, kind, key string) ([]rowHit, error) {
	table, column, err := domainTable(kind)
	if err != nil {
		return nil, err
	}
	query := fmt.Sprintf(`
		SELECT d.id, d.owner_id, COALESCE(p.username, g.github_login, ''),
		       COALESCE(gr.level, '')
		FROM %s AS d
		LEFT JOIN account_password AS p ON p.account_id = d.owner_id
		LEFT JOIN account_github AS g ON g.account_id = d.owner_id
		LEFT JOIN domain_grant AS gr
		  ON gr.domain_kind = $1 AND gr.domain_id = d.id AND gr.grantee_id = $2
		WHERE d.%s = $3`, table, column)
	rows, err := db.Query(ctx, query, kind, caller.ID, key)
	if err != nil {
		return nil, fmt.Errorf("resolve %s: %w", kind, err)
	}
	defer rows.Close()
	hits := []rowHit{}
	for rows.Next() {
		var hit rowHit
		if err := rows.Scan(&hit.id, &hit.ownerID, &hit.ownerLogin, &hit.grantLevel); err != nil {
			return nil, fmt.Errorf("resolve %s: %w", kind, err)
		}
		hits = append(hits, hit)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("resolve %s: %w", kind, err)
	}
	return hits, nil
}

func loadHitByID(ctx context.Context, db postgres.Query, caller Account, kind, id string) (rowHit, error) {
	table, _, err := domainTable(kind)
	if err != nil {
		return rowHit{}, err
	}
	query := fmt.Sprintf(`
		SELECT d.id, d.owner_id, COALESCE(p.username, g.github_login, ''),
		       COALESCE(gr.level, '')
		FROM %s AS d
		LEFT JOIN account_password AS p ON p.account_id = d.owner_id
		LEFT JOIN account_github AS g ON g.account_id = d.owner_id
		LEFT JOIN domain_grant AS gr
		  ON gr.domain_kind = $1 AND gr.domain_id = d.id AND gr.grantee_id = $2
		WHERE d.id = $3`, table)
	var hit rowHit
	err = db.QueryRow(ctx, query, kind, caller.ID, id).Scan(&hit.id, &hit.ownerID, &hit.ownerLogin, &hit.grantLevel)
	if err != nil {
		return rowHit{}, err
	}
	return hit, nil
}

func domainTable(kind string) (string, string, error) {
	switch kind {
	case DomainService:
		return "service", "image", nil
	case DomainBacktest:
		return "backtest", "name", nil
	case DomainStrategy:
		return "strategy", "name", nil
	default:
		return "", "", fmt.Errorf("unknown domain %s", kind)
	}
}

// ResolveGrant returns the domain an editor owns, or the row an admin may grant on.
// A granted editor is not found. A demoted owner is forbidden.
func ResolveGrant(ctx context.Context, db postgres.Query, caller Account, kind, key string) (string, error) {
	if caller.ID == "" {
		return "", errors.New("account id is empty")
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return "", ErrNotFound
	}
	if caller.Role == RoleAdmin {
		return adminGrantTarget(ctx, db, caller, kind, key)
	}
	id, err := ownDomain(ctx, db, caller.ID, kind, key)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", err
	}
	if caller.Role != RoleEditor {
		return "", ErrForbidden
	}
	return id, nil
}

func ownDomain(ctx context.Context, db postgres.Query, ownerID, kind, key string) (string, error) {
	table, column, err := domainTable(kind)
	if err != nil {
		return "", err
	}
	query := fmt.Sprintf(`SELECT id FROM %s WHERE owner_id = $1 AND %s = $2`, table, column)
	var id string
	err = db.QueryRow(ctx, query, ownerID, key).Scan(&id)
	if err != nil {
		return "", err
	}
	return id, nil
}

func adminGrantTarget(ctx context.Context, db postgres.Query, caller Account, kind, key string) (string, error) {
	table, column, err := domainTable(kind)
	if err != nil {
		return "", err
	}
	query := fmt.Sprintf(`SELECT id, owner_id FROM %s WHERE %s = $1`, table, column)
	rows, err := db.Query(ctx, query, key)
	if err != nil {
		return "", fmt.Errorf("grant target: %w", err)
	}
	defer rows.Close()
	type pair struct{ id, owner string }
	var owned string
	others := []string{}
	for rows.Next() {
		var item pair
		if err := rows.Scan(&item.id, &item.owner); err != nil {
			return "", fmt.Errorf("grant target: %w", err)
		}
		if item.owner == caller.ID {
			owned = item.id
			continue
		}
		others = append(others, item.id)
	}
	if err := rows.Err(); err != nil {
		return "", fmt.Errorf("grant target: %w", err)
	}
	if owned != "" {
		return owned, nil
	}
	if len(others) == 1 {
		return others[0], nil
	}
	if len(others) > 1 {
		return "", ErrNameAmbiguous
	}
	return "", ErrNotFound
}
