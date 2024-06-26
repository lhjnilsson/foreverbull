package backtest

import (
	"context"
	"fmt"

	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/entity"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/repository"
	service "github.com/lhjnilsson/foreverbull/pkg/service/entity"
	"github.com/lhjnilsson/foreverbull/pkg/service/socket"
	"github.com/lhjnilsson/foreverbull/pkg/service/worker"
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
	storedBacktest *entity.Backtest, storedSession *entity.Session, backtestInstance *service.Instance, workerPool worker.Pool,
	executions *repository.Execution, periods *repository.Period) (Session, *socket.Socket, error) {
	b, err := NewZiplineEngine(ctx, backtestInstance)
	if err != nil {
		return nil, nil, fmt.Errorf("error creating zipline engine: %w", err)
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
		return &ms, &sock, b.DownloadIngestion(ctx, environment.GetBacktestIngestionDefaultName())
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
		return &as, nil, b.DownloadIngestion(ctx, environment.GetBacktestIngestionDefaultName())

	}
}
