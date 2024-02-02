package dependency

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/backtest/internal/backtest"
	"github.com/lhjnilsson/foreverbull/backtest/internal/repository"
	ss "github.com/lhjnilsson/foreverbull/backtest/stream"
	"github.com/lhjnilsson/foreverbull/internal/stream"
	service "github.com/lhjnilsson/foreverbull/service/entity"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

const GetBacktestSessionKey stream.Dependency = "get_backtest_session"

func GetBacktestSession(ctx context.Context, message stream.Message) (interface{}, error) {
	dbConn := message.MustGet(stream.DBDep).(*pgxpool.Pool)
	log := message.MustGet(stream.LoggerDep).(*zap.Logger)

	command := ss.SessionRunCommand{}
	err := message.ParsePayload(&command)
	if err != nil {
		return nil, err
	}
	http := message.MustGet(GetHTTPClientKey).(*HTTPClient)
	instances := make(chan service.Instance, len(command.WorkerInstanceIDs)+1)
	instanceIDs := append([]string{command.BacktestInstanceID}, command.WorkerInstanceIDs...)

	g, gctx := errgroup.WithContext(ctx)
	for _, id := range instanceIDs {
		i := id
		g.Go(func() error {
			instance := service.Instance{}
			err := http.Get(gctx, fmt.Sprintf("service/api/instances/%s", i), &instance)
			if err != nil {
				return err
			}
			log.Info("Instance", zap.String("id", i), zap.Any("instance", instance))
			instances <- instance
			return nil
		})
	}
	err = g.Wait()
	if err != nil {
		return nil, err
	}
	close(instances)

	var backtestInstance *service.Instance
	var workerInstances []*service.Instance
	for instance := range instances {
		i := instance
		switch *i.ServiceType {
		case "backtest":
			backtestInstance = &i
		case "worker":
			workerInstances = append(workerInstances, &i)
		}
	}

	if backtestInstance == nil {
		return nil, fmt.Errorf("backtest instance is missing")
	}
	if len(instances) > 1 && len(workerInstances) == 0 {
		return nil, fmt.Errorf("worker instances are missing")
	}

	backtestStorage := repository.Backtest{Conn: dbConn}
	sessionStorage := repository.Session{Conn: dbConn}
	executionStorage := repository.Execution{Conn: dbConn}
	periodStorage := repository.Period{Conn: dbConn}
	orderStorage := repository.Order{Conn: dbConn}
	portfolioStorage := repository.Portfolio{Conn: dbConn}

	storedSession, err := sessionStorage.Get(ctx, command.SessionID)
	if err != nil {
		return nil, err
	}
	storedBacktest, err := backtestStorage.Get(ctx, storedSession.Backtest)
	if err != nil {
		return nil, err
	}

	s, err := backtest.NewSession(ctx, log, storedBacktest, storedSession, backtestInstance,
		&executionStorage, &periodStorage, &orderStorage, &portfolioStorage, workerInstances...)
	if err != nil {
		return nil, fmt.Errorf("error creating session: %w", err)
	}
	return s, nil
}
