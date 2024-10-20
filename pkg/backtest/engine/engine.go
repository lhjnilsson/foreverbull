package engine

import (
	"context"

	"github.com/lhjnilsson/foreverbull/internal/storage"
	finance_pb "github.com/lhjnilsson/foreverbull/pkg/finance/pb"

	"github.com/lhjnilsson/foreverbull/pkg/backtest/pb"
	"github.com/lhjnilsson/foreverbull/pkg/service/worker"
)

type Engine interface {
	Ingest(ctx context.Context, ingestion *pb.Ingestion, object *storage.Object) error
	DownloadIngestion(ctx context.Context, object *storage.Object) error
	RunBacktest(ctx context.Context, backtest *pb.Backtest, workers worker.Pool) (chan *finance_pb.Portfolio, error)
	GetResult(ctx context.Context) (*pb.GetResultResponse, error)
	Stop(ctx context.Context) error
}
