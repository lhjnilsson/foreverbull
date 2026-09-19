package api

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/pkg/finance/supplier"
	pb "github.com/lhjnilsson/foreverbull/pkg/pb/strategy"
	servicePb "github.com/lhjnilsson/foreverbull/pkg/pb/strategy/service"
	"github.com/lhjnilsson/foreverbull/pkg/strategy/internal/stream"
)

// DeploymentRunner starts and stops in-process schedule loops.
type DeploymentRunner interface {
	Add(ctx context.Context, d *pb.Deployment) error
	Remove(ctx context.Context, deploymentID string) error
}

// StrategyServer implements lifecycle RPCs.
type StrategyServer struct {
	servicePb.UnimplementedStrategyServicerServer

	pgx      *pgxpool.Pool
	launcher stream.ExperimentLauncher
	runner   DeploymentRunner
	trading  supplier.Trading
}

func NewStrategyServer(pgx *pgxpool.Pool, launcher stream.ExperimentLauncher, runner DeploymentRunner) *StrategyServer {
	return &StrategyServer{
		pgx:      pgx,
		launcher: launcher,
		runner:   runner,
	}
}

// WithTrading enables paper PlaceOrder on Deploy when trading is non-nil.
// Production only calls this when TRADING_ENABLED is true.
func (s *StrategyServer) WithTrading(trading supplier.Trading) *StrategyServer {
	s.trading = trading
	return s
}
