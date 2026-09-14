package strategy

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/stream"
	"github.com/lhjnilsson/foreverbull/pkg/backtest"
	"github.com/lhjnilsson/foreverbull/pkg/finance/supplier"
	servicePb "github.com/lhjnilsson/foreverbull/pkg/pb/strategy/service"
	"github.com/lhjnilsson/foreverbull/pkg/strategy/internal/api"
	"github.com/lhjnilsson/foreverbull/pkg/strategy/internal/repository"
	"github.com/lhjnilsson/foreverbull/pkg/strategy/internal/runner"
	ss "github.com/lhjnilsson/foreverbull/pkg/strategy/internal/stream"
	"go.uber.org/fx"
	"google.golang.org/grpc"
)

type runnerParams struct {
	fx.In

	Store   runner.Store
	Clock   runner.Clock
	Trading supplier.Trading `optional:"true"`
}

var Module = fx.Options( //nolint: gochecknoglobals
	fx.Provide(
		func(conn *pgxpool.Pool) backtest.Store {
			return backtest.NewStore(conn)
		},
		func(store backtest.Store, eventStream stream.Stream) ss.ExperimentLauncher {
			return ss.NewSessionLauncher(store, eventStream)
		},
		func(conn *pgxpool.Pool) runner.Clock {
			return runner.NewCalendarClock(conn)
		},
		func(conn *pgxpool.Pool) runner.Store {
			return runner.NewPostgresStore(conn)
		},
		func(p runnerParams) *runner.InProcess {
			return runner.New(p.Store, p.Clock, p.Trading, nil)
		},
		func(pgx *pgxpool.Pool, launcher ss.ExperimentLauncher, r *runner.InProcess) *api.StrategyServer {
			return api.NewStrategyServer(pgx, launcher, r)
		},
	),
	fx.Invoke(
		func(conn *pgxpool.Pool) error {
			return repository.CreateTables(context.Background(), conn)
		},
		func(s *grpc.Server, srv *api.StrategyServer) {
			servicePb.RegisterStrategyServicerServer(s, srv)
		},
		func(lc fx.Lifecycle, r *runner.InProcess) {
			lc.Append(fx.Hook{
				OnStart: func(ctx context.Context) error {
					if err := r.Start(ctx); err != nil {
						return fmt.Errorf("start strategy runner: %w", err)
					}
					return nil
				},
				OnStop: func(ctx context.Context) error {
					if err := r.Stop(ctx); err != nil {
						return fmt.Errorf("stop strategy runner: %w", err)
					}
					return nil
				},
			})
		},
	),
)
