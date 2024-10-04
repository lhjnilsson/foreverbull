package engine

import (
	"context"

	"github.com/lhjnilsson/foreverbull/internal/storage"
	finance_pb "github.com/lhjnilsson/foreverbull/pkg/finance/pb"

	"github.com/lhjnilsson/foreverbull/pkg/backtest/pb"
	"github.com/lhjnilsson/foreverbull/pkg/service/worker"
)

type Engine interface {
	Ingest(context.Context, *pb.Ingestion, *storage.Object) error
	DownloadIngestion(context.Context, *storage.Object) error
	RunBacktest(context.Context, *pb.Backtest, worker.Pool) (chan *finance_pb.Portfolio, error)
	GetResult(ctx context.Context) (*pb.GetResultResponse, error)
	Stop(context.Context) error
}
