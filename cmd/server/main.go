package main

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/backtest"
	"github.com/lhjnilsson/foreverbull/finance"
	"github.com/lhjnilsson/foreverbull/internal/config"
	"github.com/lhjnilsson/foreverbull/internal/http"
	"github.com/lhjnilsson/foreverbull/internal/log"
	"github.com/lhjnilsson/foreverbull/internal/storage"
	"github.com/lhjnilsson/foreverbull/internal/stream"
	"github.com/lhjnilsson/foreverbull/service"
	"github.com/lhjnilsson/foreverbull/strategy"

	"go.uber.org/fx"
)

var CoreModules = fx.Options(
	fx.Provide(
		config.GetConfig,
		func(config config.Config) (*pgxpool.Pool, error) {
			return pgxpool.New(context.TODO(), config.PostgresURI)
		},
		storage.NewMinioStorage,
		log.NewLogger,
		stream.NewJetstream,
		http.NewEngine,
	),
	fx.Invoke(
		http.NewLifeCycleRouter,
	),
)

func app() *fx.App {
	return fx.New(
		CoreModules,
		backtest.Module,
		finance.Module,
		service.Module,
		strategy.Module,
		stream.OrchestrationLifecycle,
	)
}

func main() {
	app().Run()
}
