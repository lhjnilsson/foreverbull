package api

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/pkg/finance/supplier"
	pb "github.com/lhjnilsson/foreverbull/pkg/pb/strategy"
	servicePb "github.com/lhjnilsson/foreverbull/pkg/pb/strategy/service"
	"github.com/lhjnilsson/foreverbull/pkg/strategy/internal/repository"
	"github.com/lhjnilsson/foreverbull/pkg/strategy/internal/stream"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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
	profiles supplier.Registry
}

func NewStrategyServer(pgx *pgxpool.Pool, launcher stream.ExperimentLauncher, runner DeploymentRunner) *StrategyServer {
	return &StrategyServer{
		pgx:      pgx,
		launcher: launcher,
		runner:   runner,
	}
}

// WithProfiles resolves Deployment.backend to a trading client.
// Paper PlaceOrder still requires TRADING_ENABLED and a known profile name.
func (s *StrategyServer) WithProfiles(profiles supplier.Registry) *StrategyServer {
	s.profiles = profiles
	return s
}

func (s *StrategyServer) ListStrategies(ctx context.Context, _ *servicePb.ListStrategiesRequest) (*servicePb.ListStrategiesResponse, error) {
	if s.pgx == nil {
		return nil, status.Error(codes.Internal, "strategy storage is not configured")
	}
	if environment.GetAuthEnabled() {
		return s.listStrategiesAuthed(ctx)
	}

	strategies := repository.Strategy{Conn: s.pgx}
	list, err := strategies.List(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list strategies: %v", err)
	}

	return &servicePb.ListStrategiesResponse{Strategies: list}, nil
}

func (s *StrategyServer) GetStrategy(ctx context.Context, req *servicePb.GetStrategyRequest) (*servicePb.GetStrategyResponse, error) {
	if s.pgx == nil {
		return nil, status.Error(codes.Internal, "strategy storage is not configured")
	}
	if environment.GetAuthEnabled() {
		return s.getStrategyAuthed(ctx, req)
	}

	strategies := repository.Strategy{Conn: s.pgx}
	found, err := strategies.Get(ctx, req.GetName())
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, status.Errorf(codes.NotFound, "strategy %q not found", req.GetName())
		}
		return nil, status.Errorf(codes.Internal, "get strategy: %v", err)
	}

	return &servicePb.GetStrategyResponse{Strategy: found}, nil
}

func (s *StrategyServer) DeleteStrategy(ctx context.Context, req *servicePb.DeleteStrategyRequest) (*servicePb.DeleteStrategyResponse, error) {
	if !environment.GetAuthEnabled() {
		return s.UnimplementedStrategyServicerServer.DeleteStrategy(ctx, req)
	}
	return s.deleteStrategyAuthed(ctx, req)
}
