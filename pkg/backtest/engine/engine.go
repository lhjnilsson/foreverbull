package engine

import (
	"context"

	"github.com/lhjnilsson/foreverbull/internal/storage"
	"github.com/lhjnilsson/foreverbull/pkg/pb/backtest"
	servicePb "github.com/lhjnilsson/foreverbull/pkg/pb/backtest/service"
	"github.com/lhjnilsson/foreverbull/pkg/service/worker"
)

type Engine interface {
	// Ingest builds the bundle. notices are lines the user must read even
	// though the ingest worked, such as sessions filled with the previous close.
	Ingest(ctx context.Context, backtest *backtest.Backtest, object *storage.Object) (notices []string, err error)
	DownloadIngestion(ctx context.Context, object *storage.Object) error
	NewSession(ctx context.Context, session *backtest.Session) (EngineSession, error)
}

type EngineSession interface {
	RunBacktest(ctx context.Context, execution *backtest.Execution, workers worker.Pool) (chan *servicePb.RunExecutionResponse, error)
	GetResult(ctx context.Context, executionId string) (*servicePb.GetResultResponse, error)
}
