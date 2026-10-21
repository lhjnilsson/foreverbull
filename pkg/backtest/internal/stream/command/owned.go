package command

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/lhjnilsson/foreverbull/internal/auth"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/repository"
	"github.com/lhjnilsson/foreverbull/pkg/pb/backtest"
)

func requireStoredOwner(ownerID, backtestID string) error {
	if !environment.GetAuthEnabled() {
		return nil
	}
	if strings.TrimSpace(ownerID) == "" && strings.TrimSpace(backtestID) == "" {
		return errors.New("owner id is empty")
	}
	return nil
}

func loadStoredBacktest(ctx context.Context, pool *pgxpool.Pool, name, ownerID, backtestID string) (*backtest.Backtest, string, error) {
	backtests := repository.Backtest{Conn: pool}
	if !environment.GetAuthEnabled() {
		item, err := backtests.Get(ctx, name)
		if err != nil {
			return nil, "", err
		}
		return item, "", nil
	}
	if err := requireStoredOwner(ownerID, backtestID); err != nil {
		return nil, "", err
	}
	id := strings.TrimSpace(backtestID)
	if id == "" {
		var err error
		id, err = backtests.IDByOwnerAndName(ctx, ownerID, name)
		if err != nil {
			return nil, "", fmt.Errorf("error getting backtest: %w", err)
		}
	}
	item, err := backtests.GetByID(ctx, id)
	if err != nil {
		return nil, "", err
	}
	return item, id, nil
}

func deleteStoredBacktest(ctx context.Context, backtests repository.Backtest, name, id string) error {
	if environment.GetAuthEnabled() {
		if id == "" {
			return errors.New("backtest id is empty")
		}
		return backtests.DeleteByID(ctx, id)
	}
	return backtests.Delete(ctx, name)
}

func uniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func createStoredBacktest(ctx context.Context, backtests repository.Backtest, ownerID string, item *backtest.Backtest) error {
	if environment.GetAuthEnabled() {
		if strings.TrimSpace(ownerID) == "" {
			return errors.New("owner id is empty")
		}
		_, err := backtests.CreateOwned(ctx, ownerID, item)
		if uniqueViolation(err) {
			return fmt.Errorf("error creating backtest: %w", auth.ErrAlreadyExists)
		}
		return err
	}
	_, err := backtests.Create(ctx, item)
	return err
}
