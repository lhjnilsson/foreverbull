package command

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/repository"
	"github.com/lhjnilsson/foreverbull/pkg/finance"
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
		id, err := backtests.IDByName(ctx, name)
		if err != nil {
			return nil, "", fmt.Errorf("error getting backtest id: %w", err)
		}
		return item, id, nil
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
	if strings.TrimSpace(id) == "" {
		return nil, "", errors.New("backtest id is empty")
	}
	var storedOwner string
	err := pool.QueryRow(ctx, `SELECT owner_id FROM backtest WHERE id = $1`, id).Scan(&storedOwner)
	if err != nil {
		return nil, "", err
	}
	// The message owner is not the authority. A different id is a forged command.
	if strings.TrimSpace(ownerID) != storedOwner {
		return nil, "", errOwnerMismatch
	}
	item, err := backtests.GetByID(ctx, id)
	if err != nil {
		return nil, "", err
	}
	return item, id, nil
}

var errOwnerMismatch = errors.New("backtest owner does not match")

func deleteStoredBacktest(ctx context.Context, backtests repository.Backtest, name, id string) error {
	if environment.GetAuthEnabled() {
		if id == "" {
			return errors.New("backtest id is empty")
		}
		return backtests.DeleteByID(ctx, id)
	}
	return backtests.Delete(ctx, name)
}

func createStoredBacktest(ctx context.Context, backtests repository.Backtest, ownerID string, item *backtest.Backtest) error {
	return storeBacktest(ctx, backtests, ownerID, item, nil, false)
}

func createStoredBacktestWithMembers(ctx context.Context, backtests repository.Backtest, ownerID string, item *backtest.Backtest, members []repository.MemberInterval) error {
	return storeBacktest(ctx, backtests, ownerID, item, members, true)
}

// storeBacktest inserts the backtest and keeps the id the insert returned.
// A second create of the same name is ErrBacktestNameTaken. The stored row
// is not loaded again by name.
func storeBacktest(ctx context.Context, backtests repository.Backtest, ownerID string, item *backtest.Backtest, members []repository.MemberInterval, writeMembers bool) error {
	var (
		id  string
		err error
	)
	if environment.GetAuthEnabled() {
		if strings.TrimSpace(ownerID) == "" {
			return errors.New("owner id is empty")
		}
		if writeMembers {
			id, _, err = backtests.CreateOwnedWithMembers(ctx, ownerID, item, members)
		} else {
			id, _, err = backtests.CreateOwned(ctx, ownerID, item)
		}
	} else if writeMembers {
		id, _, err = backtests.CreateWithMembers(ctx, item, members)
	} else {
		id, _, err = backtests.Create(ctx, item)
	}
	if err != nil {
		return err
	}
	if strings.TrimSpace(id) == "" {
		return errors.New("backtest id is empty")
	}
	// The listing check already passed. A later download, ingest, or run
	// reads this and does not ask the provider again. An unknown symbol
	// did not answer, so the row stays unchecked.
	symbols, benchmark := backtestMembers(item)
	asked := append(append([]string{}, symbols...), benchmark)
	if !finance.ListingsAnswered(item.GetProvider(), asked) {
		return nil
	}
	if err := backtests.MarkMarketsChecked(ctx, item.GetName(), id); err != nil {
		return err
	}
	return nil
}
