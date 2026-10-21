package strategy

import (
	"context"
	"fmt"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/stream"
	"github.com/lhjnilsson/foreverbull/pkg/backtest"
	"github.com/lhjnilsson/foreverbull/pkg/finance/supplier"
	servicePb "github.com/lhjnilsson/foreverbull/pkg/pb/strategy/service"
	"github.com/lhjnilsson/foreverbull/pkg/service"
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

	Store    runner.Store
	Clock    runner.Clock
	Profiles supplier.Registry `optional:"true"`
	Pool     worker.Pool       `optional:"true"`
}

type serverParams struct {
	fx.In

	Pgx      *pgxpool.Pool
	Launcher ss.ExperimentLauncher
	Runner   *runner.InProcess
	Profiles supplier.Registry `optional:"true"`
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
			return runner.New(p.Store, p.Clock, p.Profiles, processor)
		},
		func(p serverParams) *api.StrategyServer {
			return api.NewStrategyServer(p.Pgx, p.Launcher, p.Runner).WithProfiles(p.Profiles)
		},
	),
	fx.Invoke(
		func(conn *pgxpool.Pool) error {
			return repository.CreateTables(context.Background(), conn)
		},
		func(s *grpc.Server, srv *api.StrategyServer) {
			servicePb.RegisterStrategyServicerServer(s, srv)
		},
		registerStrategyHTTP,
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

type strategyHTTPParams struct {
	fx.In

	Mux      *http.ServeMux `optional:"true"`
	Server   *api.StrategyServer
	Pgx      *pgxpool.Pool         `optional:"true"`
	Launcher ss.ExperimentLauncher `optional:"true"`
}

// registerStrategyHTTP mounts routes when the HTTP mux is in the process.
// Module tests that do not provide the mux keep gRPC and do not listen on HTTP_PORT.
// POST /api/strategies uses the strategy repository. CreateStrategy stays unimplemented.
// POST /api/backtests/{name}/executions loads the stored service and calls the launcher.
// The handler does not dial session gRPC.
func registerStrategyHTTP(p strategyHTTPParams) {
	if p.Mux == nil || p.Server == nil {
		return
	}
	var writer api.StrategyWriter
	if p.Pgx != nil {
		writer = &repository.Strategy{Conn: p.Pgx}
	}
	api.RegisterHTTP(p.Mux, p.Server, writer)
	if p.Pgx == nil || p.Launcher == nil {
		return
	}
	api.RegisterExecutionHTTPWithAccess(p.Mux, service.NewStore(p.Pgx), p.Launcher, p.Pgx)
}
