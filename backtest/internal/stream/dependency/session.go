package dependency

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/backtest/internal/backtest"
	"github.com/lhjnilsson/foreverbull/backtest/internal/repository"
	ss "github.com/lhjnilsson/foreverbull/backtest/stream"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/internal/stream"
	serviceAPI "github.com/lhjnilsson/foreverbull/service/api"
	service "github.com/lhjnilsson/foreverbull/service/entity"
	"github.com/lhjnilsson/foreverbull/service/worker"
	"golang.org/x/sync/errgroup"
)

const GetBacktestSessionKey stream.Dependency = "get_backtest_session"

func GetBacktestSession(ctx context.Context, message stream.Message) (interface{}, error) {
	dbConn := message.MustGet(stream.DBDep).(*pgxpool.Pool)
	backtestStorage := repository.Backtest{Conn: dbConn}
	sessionStorage := repository.Session{Conn: dbConn}
	executionStorage := repository.Execution{Conn: dbConn}
	periodStorage := repository.Period{Conn: dbConn}

	command := ss.SessionRunCommand{}
	err := message.ParsePayload(&command)
	if err != nil {
		return nil, err
	}
	sAPI := message.MustGet(GetServiceAPI).(serviceAPI.Client)

	b, err := backtestStorage.Get(ctx, command.Backtest)
	if err != nil {
		return nil, err
	}
	var backtestInstance *service.Instance
	var pool worker.Pool

	if b.Service != nil {
		s, err := sAPI.GetService(ctx, *b.Service)
		if err != nil {
			return nil, err
		}
		functions, err := s.Algorithm.Configure()
		if err != nil {
			return nil, err
		}
		pool, err = worker.NewPool(ctx, s.Algorithm)
		if err != nil {
			return nil, err
		}

		configure := serviceAPI.ConfigureInstanceRequest{
			BrokerPort:  pool.GetPort(),
			DatabaseURL: environment.GetPostgresURL(),
			Functions:   functions,
		}
		g, gctx := errgroup.WithContext(ctx)
		for _, id := range command.WorkerInstanceIDs {
			i := id
			g.Go(func() error {
				_, err = sAPI.ConfigureInstance(gctx, i, &configure)
				if err != nil {
					return err
				}
				return nil
			})
		}
		g.Go(func() error {
			var err error
			instance, err := sAPI.GetInstance(gctx, command.BacktestInstanceID)
			if err != nil {
				return err
			}
			b := service.Instance(*instance)
			backtestInstance = &b
			return nil
		})
		err = g.Wait()
		if err != nil {
			return nil, err
		}
	}
	if backtestInstance == nil {
		return nil, fmt.Errorf("backtest instance is missing")
	}

	storedSession, err := sessionStorage.Get(ctx, command.SessionID)
	if err != nil {
		return nil, err
	}
	storedBacktest, err := backtestStorage.Get(ctx, storedSession.Backtest)
	if err != nil {
		return nil, err
	}

	s, socket, err := backtest.NewSession(ctx, storedBacktest, storedSession, backtestInstance, pool,
		&executionStorage, &periodStorage)
	if err != nil {
		return nil, fmt.Errorf("error creating session: %w", err)
	}
	if socket != nil {
		err = sessionStorage.UpdatePort(ctx, storedSession.ID, socket.Port)
		if err != nil {
			return nil, err
		}
	}
	return s, nil
}
