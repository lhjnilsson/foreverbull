package strategy

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/internal/stream"
	"github.com/lhjnilsson/foreverbull/pkg/backtest"
	"github.com/lhjnilsson/foreverbull/pkg/finance/supplier"
	servicePb "github.com/lhjnilsson/foreverbull/pkg/pb/strategy/service"
	"github.com/lhjnilsson/foreverbull/pkg/service/worker"
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
	Pool    worker.Pool      `optional:"true"`
}

type serverParams struct {
	fx.In

	Pgx      *pgxpool.Pool
	Launcher ss.ExperimentLauncher
	Runner   *runner.InProcess
	Trading  supplier.Trading `optional:"true"`
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
			var processor runner.Processor
			if p.Pool != nil {
				processor = p.Pool
			}
			return runner.New(p.Store, p.Clock, p.Trading, processor)
		},
		func(p serverParams) *api.StrategyServer {
			srv := api.NewStrategyServer(p.Pgx, p.Launcher, p.Runner)
			if environment.GetTradingEnabled() && p.Trading != nil {
				return srv.WithTrading(p.Trading)
			}
			return srv
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
