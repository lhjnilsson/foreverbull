package backtest

import (
	"context"
	"fmt"

	"github.com/lhjnilsson/foreverbull/backtest/entity"
	"github.com/lhjnilsson/foreverbull/backtest/internal/repository"
	"github.com/lhjnilsson/foreverbull/service/backtest"
	service "github.com/lhjnilsson/foreverbull/service/entity"
	"github.com/lhjnilsson/foreverbull/service/socket"
	"github.com/lhjnilsson/foreverbull/service/worker"
)

type Session interface {
	Run(chan<- bool, <-chan bool) error
	Stop(ctx context.Context) error
}

type session struct {
	backtest *entity.Backtest `json:"-"`
	session  *entity.Session

	executions *repository.Execution `json:"-"`
	periods    *repository.Period    `json:"-"`
}

func NewSession(ctx context.Context,
	storedBacktest *entity.Backtest, storedSession *entity.Session, backtestInstance *service.Instance,
	executions *repository.Execution, periods *repository.Period, workers ...*service.Instance) (Session, *socket.Socket, error) {
	b, err := backtest.NewZiplineEngine(ctx, backtestInstance)
	if err != nil {
		return nil, nil, fmt.Errorf("error creating zipline engine: %w", err)
	}
	workerPool, err := worker.NewPool(ctx, workers...)
	if err != nil {
		return nil, nil, fmt.Errorf("error creating worker pool: %w", err)
	}
	s := session{
		backtest: storedBacktest,
		session:  storedSession,

		executions: executions,
		periods:    periods,
	}

	if storedSession.Manual {
		sock := socket.Socket{Type: socket.Replier, Host: "0.0.0.0", Port: 0, Listen: true, Dial: false}
		socket, err := socket.GetContextSocket(ctx, &sock)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to get socket: %w", err)
		}

		ms := manualSession{
			session: s,

			backtest: b,
			workers:  workerPool,

			Socket: sock,
			socket: socket,
		}
		return &ms, &sock, b.DownloadIngestion(ctx, storedBacktest.Name)
	} else {
		executions, err := executions.ListBySession(ctx, storedSession.ID)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to list executions: %w", err)
		}

		as := automatedSession{
			session: s,

			backtest: b,
			workers:  workerPool,

			executions: executions,
		}
		return &as, nil, b.DownloadIngestion(ctx, storedBacktest.Name)

	}
}
