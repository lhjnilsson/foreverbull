package engine

import (
	"context"

	"github.com/lhjnilsson/foreverbull/internal/storage"
	pb "github.com/lhjnilsson/foreverbull/pkg/pb/backtest"
	servicePb "github.com/lhjnilsson/foreverbull/pkg/pb/backtest/service"
	"github.com/lhjnilsson/foreverbull/pkg/service/worker"
)

type Engine interface {
	Ingest(ctx context.Context, ingestion *servicePb.Ingestion, object *storage.Object) error
	DownloadIngestion(ctx context.Context, object *storage.Object) error
	NewSession(ctx context.Context, session *pb.Session) (EngineSession, error)
}

type EngineSession interface {
	RunBacktest(ctx context.Context, backtest *pb.Backtest, workers worker.Pool) (chan *servicePb.RunExecutionResponse, error)
	GetResult(ctx context.Context, executionId string) (*servicePb.GetResultResponse, error)
}
