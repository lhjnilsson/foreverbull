package main

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/lhjnilsson/foreverbull/internal/auth"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/internal/storage"
	"github.com/lhjnilsson/foreverbull/internal/web"
	"github.com/lhjnilsson/foreverbull/pkg/backtest"
	"github.com/lhjnilsson/foreverbull/pkg/service"
	"github.com/lhjnilsson/foreverbull/pkg/strategy"
)

// prepareAuth runs before the process accepts requests. A false flag keeps
// today's schema. A true flag creates the admin, migrates owners, and then
// requires a credential.
func prepareAuth(pool *pgxpool.Pool, store storage.Storage, mux *http.ServeMux) error {
	adminID, err := auth.Startup(context.Background(), pool, log.Printf)
	if err != nil {
		return err
	}
	if !environment.GetAuthEnabled() {
		return nil
	}
	if err := service.MigrateOwner(context.Background(), pool, adminID); err != nil {
		return err
	}
	copier, err := ingestionCopier(store)
	if err != nil {
		return err
	}
	if err := backtest.MigrateOwner(context.Background(), pool, adminID, copier); err != nil {
		return err
	}
	if err := strategy.MigrateOwner(context.Background(), pool, adminID); err != nil {
		return err
	}
	auth.RegisterHTTP(mux, pool)
	web.UseGuard(auth.Protect(pool))
	go auth.SweepLoop(context.Background(), pool, log.Printf)
	return nil
}

func ingestionCopier(store storage.Storage) (backtest.IngestionCopier, error) {
	client, ok := store.(*storage.MinioStorage)
	if !ok || client == nil {
		return nil, fmt.Errorf("%s: ingestion copier is nil", storage.IngestionsBucket)
	}
	return minioIngestion{client: client}, nil
}

type minioIngestion struct {
	client *storage.MinioStorage
}

func (m minioIngestion) Readable(ctx context.Context, key string) (bool, error) {
	return m.client.IngestionReadable(ctx, key)
}

func (m minioIngestion) Copy(ctx context.Context, fromKey, toKey string) error {
	return m.client.CopyIngestion(ctx, fromKey, toKey)
}
