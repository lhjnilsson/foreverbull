package api

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
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
}

func NewStrategyServer(pgx *pgxpool.Pool, launcher stream.ExperimentLauncher, runner DeploymentRunner) *StrategyServer {
	return &StrategyServer{
		pgx:      pgx,
		launcher: launcher,
		runner:   runner,
	}
}
