package api

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	pb "github.com/lhjnilsson/foreverbull/pkg/pb/strategy"
	servicePb "github.com/lhjnilsson/foreverbull/pkg/pb/strategy/service"
	"github.com/lhjnilsson/foreverbull/pkg/strategy/internal/repository"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	defaultBackend  = "default"
	defaultCalendar = "XNYS"
)

func (s *StrategyServer) Deploy(ctx context.Context, req *servicePb.DeployRequest) (*servicePb.DeployResponse, error) {
	if s.pgx == nil {
		return nil, status.Error(codes.Internal, "strategy storage is not configured")
	}

	if req.GetMode() == pb.Deployment_LIVE {
		return nil, status.Error(codes.FailedPrecondition,
			"live mode is not enabled; deploy with paper")
	}

	dryRun := true
	if req.DryRun != nil {
		dryRun = *req.DryRun
	}
	if !dryRun {
		if !req.GetConfirm() {
			return nil, status.Error(codes.FailedPrecondition,
				"confirm is required when dry_run=false")
		}
		if s.trading == nil {
			return nil, status.Error(codes.FailedPrecondition,
				"paper PlaceOrder requires TRADING_ENABLED and a configured Trading backend")
		}
	}

	version, err := s.resolveVersion(ctx, req.GetStrategyName(), req.GetVersionId(), req.GetVersionRev())
	if err != nil {
		return nil, err
	}
	if err := s.requireServiceReady(ctx, version.ServiceImage); err != nil {
		return nil, err
	}
	if err := s.requireCompletedExperiment(ctx, version); err != nil {
		return nil, err
	}

	backend := req.GetBackend()
	if backend == "" {
		backend = defaultBackend
	}
	calendar := req.GetCalendarCode()
	if calendar == "" {
		calendar = defaultCalendar
	}

	deployments := repository.Deployment{Conn: s.pgx}
	deployment, err := deployments.Create(
		ctx, version.Id, backend, calendar,
		pb.Deployment_PAPER, req.GetSymbols(), pb.Deployment_SESSION_CLOSE, dryRun,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, status.Errorf(codes.FailedPrecondition,
				"strategy %q already has a running deployment; stop it first", req.GetStrategyName())
		}
		return nil, status.Errorf(codes.Internal, "create deployment: %v", err)
	}

	if s.runner != nil {
		if err := s.runner.Add(ctx, deployment); err != nil {
			_ = deployments.UpdateStatus(ctx, deployment.Id, pb.Deployment_ERROR, err)
			return nil, status.Errorf(codes.Internal, "start deployment runner: %v", err)
		}
	}

	return &servicePb.DeployResponse{Deployment: deployment}, nil
}

func (s *StrategyServer) StopDeployment(ctx context.Context, req *servicePb.StopDeploymentRequest) (*servicePb.StopDeploymentResponse, error) {
	if s.pgx == nil {
		return nil, status.Error(codes.Internal, "strategy storage is not configured")
	}

	deployments := repository.Deployment{Conn: s.pgx}
	var deployment *pb.Deployment
	var err error

	switch {
	case req.GetDeploymentId() != "":
		deployment, err = deployments.Get(ctx, req.GetDeploymentId())
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, status.Errorf(codes.NotFound, "deployment %q not found", req.GetDeploymentId())
			}
			return nil, status.Errorf(codes.Internal, "get deployment: %v", err)
		}
		if req.GetStrategyName() != "" && deployment.StrategyName != req.GetStrategyName() {
			return nil, status.Errorf(codes.InvalidArgument,
				"deployment %q does not belong to strategy %q", req.GetDeploymentId(), req.GetStrategyName())
		}
	case req.GetStrategyName() != "":
		strategies := repository.Strategy{Conn: s.pgx}
		if _, err := strategies.Get(ctx, req.GetStrategyName()); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, status.Errorf(codes.NotFound, "strategy %q not found", req.GetStrategyName())
			}
			return nil, status.Errorf(codes.Internal, "get strategy: %v", err)
		}
		deployment, err = deployments.GetRunningByStrategyName(ctx, req.GetStrategyName())
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				list, listErr := deployments.List(ctx, req.GetStrategyName())
				if listErr != nil {
					return nil, status.Errorf(codes.Internal, "list deployments: %v", listErr)
				}
				if len(list) == 0 {
					return nil, status.Errorf(codes.NotFound, "strategy %q has no deployments", req.GetStrategyName())
				}
				return &servicePb.StopDeploymentResponse{Deployment: list[len(list)-1]}, nil
			}
			return nil, status.Errorf(codes.Internal, "get running deployment: %v", err)
		}
	default:
		return nil, status.Error(codes.InvalidArgument, "strategy_name or deployment_id is required")
	}

	if deployment.Status == pb.Deployment_RUNNING {
		if err := deployments.Stop(ctx, deployment.Id); err != nil {
			return nil, status.Errorf(codes.Internal, "stop deployment: %v", err)
		}
		if s.runner != nil {
			_ = s.runner.Remove(ctx, deployment.Id)
		}
		deployment, err = deployments.Get(ctx, deployment.Id)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "reload deployment: %v", err)
		}
	}

	return &servicePb.StopDeploymentResponse{Deployment: deployment}, nil
}

func (s *StrategyServer) ListDeploymentRuns(ctx context.Context, req *servicePb.ListDeploymentRunsRequest) (*servicePb.ListDeploymentRunsResponse, error) {
	if s.pgx == nil {
		return nil, status.Error(codes.Internal, "strategy storage is not configured")
	}

	runs := repository.DeploymentRun{Conn: s.pgx}
	list, err := runs.List(ctx, req.GetStrategyName(), req.GetDeploymentId())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list deployment runs: %v", err)
	}
	return &servicePb.ListDeploymentRunsResponse{Runs: list}, nil
}

func (s *StrategyServer) requireCompletedExperiment(ctx context.Context, version *pb.Version) error {
	experiments := repository.Experiment{Conn: s.pgx}
	list, err := experiments.List(ctx, version.StrategyName, version.Id, "")
	if err != nil {
		return status.Errorf(codes.Internal, "list experiments: %v", err)
	}
	for _, experiment := range list {
		if experiment.Status == pb.Experiment_COMPLETED {
			return nil
		}
	}
	return status.Errorf(codes.FailedPrecondition,
		"version %q has no completed experiment; run CreateExperiment before Deploy", version.Id)
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
