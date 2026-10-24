package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/lhjnilsson/foreverbull/internal/auth"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	internal_pb "github.com/lhjnilsson/foreverbull/pkg/pb"
	pb "github.com/lhjnilsson/foreverbull/pkg/pb/strategy"
)

// ListVisible returns strategies the caller may see.
func (db *Strategy) ListVisible(ctx context.Context, caller auth.Account) ([]*pb.Strategy, error) {
	if caller.ID == "" {
		return nil, errors.New("account id is empty")
	}
	where, args := strategyVisibleWhere("s", caller)
	rows, err := db.Conn.Query(ctx, `
		SELECT s.id, s.name, s.description, s.created_at, COALESCE(p.username, g.github_login, '')
		FROM strategy AS s
		LEFT JOIN account_password AS p ON p.account_id = s.owner_id
		LEFT JOIN account_github AS g ON g.account_id = s.owner_id
		WHERE `+where+`
		ORDER BY s.created_at ASC`, args...)
	if err != nil {
		return nil, fmt.Errorf("list visible strategies: %w", err)
	}
	defer rows.Close()
	return parseOwnedStrategies(rows)
}

func strategyVisibleWhere(alias string, caller auth.Account) (string, []any) {
	switch caller.Role {
	case auth.RoleAdmin:
		return "TRUE", nil
	case auth.RoleEditor:
		return `(` + alias + `.owner_id = $1 OR EXISTS (
			SELECT 1 FROM domain_grant AS gr
			WHERE gr.domain_kind = $2 AND gr.domain_id = ` + alias + `.id AND gr.grantee_id = $1))`,
			[]any{caller.ID, auth.DomainStrategy}
	default:
		return `EXISTS (
			SELECT 1 FROM domain_grant AS gr
			WHERE gr.domain_kind = $2 AND gr.domain_id = ` + alias + `.id AND gr.grantee_id = $1)`,
			[]any{caller.ID, auth.DomainStrategy}
	}
}

// GetOwned loads one strategy and its owner login.
func (db *Strategy) GetOwned(ctx context.Context, id string) (*pb.Strategy, error) {
	if id == "" {
		return nil, fmt.Errorf("failed to get strategy: %w", errors.New("strategy id is empty"))
	}
	row := db.Conn.QueryRow(ctx, `
		SELECT s.id, s.name, s.description, s.created_at, COALESCE(p.username, g.github_login, '')
		FROM strategy AS s
		LEFT JOIN account_password AS p ON p.account_id = s.owner_id
		LEFT JOIN account_github AS g ON g.account_id = s.owner_id
		WHERE s.id = $1`, id)
	item, err := scanOwnedStrategy(row)
	if err != nil {
		return nil, fmt.Errorf("failed to get strategy: %w", err)
	}
	return item, nil
}

// CreateOwned inserts a strategy for ownerID.
func (db *Strategy) CreateOwned(ctx context.Context, ownerID, name string, description *string) (*pb.Strategy, error) {
	if ownerID == "" {
		return nil, errors.New("account id is empty")
	}
	var id string
	err := db.Conn.QueryRow(ctx, `
		INSERT INTO strategy (name, description, owner_id) VALUES ($1, $2, $3) RETURNING id`,
		name, description, ownerID).Scan(&id)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, auth.ErrAlreadyExists
		}
		return nil, fmt.Errorf("failed to create strategy: %w", err)
	}
	return db.GetOwned(ctx, id)
}

// DeleteByID removes one strategy. It does not delete by name.
func (db *Strategy) DeleteByID(ctx context.Context, id string) error {
	if id == "" {
		return errors.New("strategy id is empty")
	}
	tag, err := db.Conn.Exec(ctx, `DELETE FROM strategy WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("failed to delete strategy: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return auth.ErrNotFound
	}
	return nil
}

func (db *Strategy) guardNameDelete() error {
	if environment.GetAuthEnabled() {
		return fmt.Errorf("delete strategy by name is refused while AUTH_ENABLED=true")
	}
	return nil
}

type strategyScanner interface {
	Scan(dest ...any) error
}

func scanOwnedStrategy(row strategyScanner) (*pb.Strategy, error) {
	item := pb.Strategy{}
	var description *string
	var created time.Time
	if err := row.Scan(&item.Id, &item.Name, &description, &created, &item.OwnerLogin); err != nil {
		return nil, err
	}
	item.Description = description
	item.CreatedAt = internal_pb.TimeToProtoTimestamp(created)
	return &item, nil
}

type strategyRows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
}

func parseOwnedStrategies(rows strategyRows) ([]*pb.Strategy, error) {
	out := []*pb.Strategy{}
	for rows.Next() {
		item, err := scanOwnedStrategy(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan strategy: %w", err)
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to list strategies: %w", err)
	}
	return out, nil
}
