package engine

import (
	"context"

	"github.com/lhjnilsson/foreverbull/internal/storage"
	pb "github.com/lhjnilsson/foreverbull/pkg/pb/backtest"
	finance_pb "github.com/lhjnilsson/foreverbull/pkg/pb/finance"
	"github.com/lhjnilsson/foreverbull/pkg/service/worker"
)

type Engine interface {
	Ingest(ctx context.Context, ingestion *pb.Ingestion, object *storage.Object) error
	DownloadIngestion(ctx context.Context, object *storage.Object) error
	NewSession(ctx context.Context, session *pb.Session) (EngineSession, error)
}

type EngineSession interface {
	RunBacktest(ctx context.Context, backtest *pb.Backtest, workers worker.Pool) (chan *finance_pb.Portfolio, error)
	GetResult(ctx context.Context) (*pb.GetResultResponse, error)
}
