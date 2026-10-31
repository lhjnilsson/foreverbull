package engine

import (
	"context"
	"time"

	"github.com/lhjnilsson/foreverbull/internal/storage"
	"github.com/lhjnilsson/foreverbull/pkg/pb/backtest"
	servicePb "github.com/lhjnilsson/foreverbull/pkg/pb/backtest/service"
	"github.com/lhjnilsson/foreverbull/pkg/service/worker"
)

type Engine interface {
	// Ingest builds the bundle. notices are lines the user must read even
	// though the ingest worked. Zipline sends none today.
	Ingest(ctx context.Context, backtest *backtest.Backtest, object *storage.Object) (notices []string, err error)
	// Sessions lists the XNYS sessions from start to end, both inclusive, as
	// UTC midnights. It reads the calendar the bundle is written with, so a
	// day it leaves out (a holiday) is not a session the bundle needs.
	Sessions(ctx context.Context, start, end time.Time) ([]time.Time, error)
	DownloadIngestion(ctx context.Context, object *storage.Object) error
	NewSession(ctx context.Context, session *backtest.Session) (EngineSession, error)
}

type EngineSession interface {
	RunBacktest(ctx context.Context, execution *backtest.Execution, workers worker.Pool) (chan *servicePb.RunExecutionResponse, error)
	GetResult(ctx context.Context, executionId string) (*servicePb.GetResultResponse, error)
}
