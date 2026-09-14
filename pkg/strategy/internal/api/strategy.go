package api

import (
	"github.com/jackc/pgx/v5/pgxpool"
	pb "github.com/lhjnilsson/foreverbull/pkg/pb/strategy/service"
	"github.com/lhjnilsson/foreverbull/pkg/strategy/internal/stream"
)

// StrategyServer implements lifecycle RPCs. Deploy/Stop are still unimplemented.
type StrategyServer struct {
	pb.UnimplementedStrategyServicerServer

	pgx      *pgxpool.Pool
	launcher stream.ExperimentLauncher
}

func NewStrategyServer(pgx *pgxpool.Pool, launcher stream.ExperimentLauncher) *StrategyServer {
	return &StrategyServer{
		pgx:      pgx,
		launcher: launcher,
	}
}
