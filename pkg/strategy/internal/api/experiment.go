package api

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	common "github.com/lhjnilsson/foreverbull/pkg/pb"
	service "github.com/lhjnilsson/foreverbull/pkg/pb/service"
	pb "github.com/lhjnilsson/foreverbull/pkg/pb/strategy"
	servicePb "github.com/lhjnilsson/foreverbull/pkg/pb/strategy/service"
	"github.com/lhjnilsson/foreverbull/pkg/strategy/internal/repository"
	"github.com/lhjnilsson/foreverbull/pkg/strategy/internal/stream"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// grafanaDashboardUID is the provisioned Backtest execution dashboard.
// Deep links must keep this UID and the var-execution query key.
const grafanaDashboardUID = "ee9ep4jxtwkcgf"

// GrafanaExecutionURL builds a Grafana inspect-one-run deep link.
// grafanaURL is the Grafana origin (GRAFANA_URL); empty uses the Kind default.
func GrafanaExecutionURL(grafanaURL, executionID string) string {
	base := strings.TrimSpace(grafanaURL)
	if base == "" {
		base = environment.GrafanaURLDefault
	}
	u, err := url.Parse(base)
	if err != nil || u.Scheme == "" || u.Host == "" {
		u, err = url.Parse(environment.GrafanaURLDefault)
		if err != nil {
			u = &url.URL{Scheme: "http", Host: "localhost:3000"}
		}
	}
	built := u.JoinPath("d", grafanaDashboardUID)
	q := built.Query()
	q.Set("var-execution", executionID)
	built.RawQuery = q.Encode()
	return built.String()
}

func (s *StrategyServer) CreateExperiment(
	req *servicePb.CreateExperimentRequest,
	serverStream servicePb.StrategyServicer_CreateExperimentServer,
) error {
	if s.pgx == nil {
		return status.Error(codes.Internal, "strategy storage is not configured")
	}
	if s.launcher == nil {
		return status.Error(codes.Internal, "experiment launcher is not configured")
	}

	ctx := serverStream.Context()
	rsp := &servicePb.CreateExperimentResponse{
		QUEUED:    &common.Step{State: common.State_STATE_UNSPECIFIED},
		RUNNING:   &common.Step{State: common.State_STATE_UNSPECIFIED},
		COMPLETED: &common.Step{State: common.State_STATE_UNSPECIFIED},
	}

	version, err := s.resolveVersion(ctx, req.GetStrategyName(), req.GetVersionId(), req.GetVersionRev())
	if err != nil {
		return err
	}

	if err := s.requireServiceReady(ctx, version.ServiceImage); err != nil {
		return err
	}
	if err := s.requireBacktest(ctx, req.GetBacktestName()); err != nil {
		return err
	}

	rsp.QUEUED.State = common.State_STATE_RUNNING
	if err := serverStream.Send(rsp); err != nil {
		return fmt.Errorf("send queued: %w", err)
	}

	experiments := repository.Experiment{Conn: s.pgx}
	experiment, err := experiments.Create(ctx, version.Id, req.GetBacktestName())
	if err != nil {
		return status.Errorf(codes.Internal, "create experiment: %v", err)
	}

	rsp.QUEUED.State = common.State_STATE_COMPLETED
	rsp.Experiment = experiment
	if err := serverStream.Send(rsp); err != nil {
		return fmt.Errorf("send queued complete: %w", err)
	}

	if err := experiments.UpdateStatus(ctx, experiment.Id, pb.Experiment_RUNNING, nil); err != nil {
		return status.Errorf(codes.Internal, "update experiment status: %v", err)
	}
	experiment.Status = pb.Experiment_RUNNING
	rsp.RUNNING.State = common.State_STATE_RUNNING
	rsp.Experiment = experiment
	if err := serverStream.Send(rsp); err != nil {
		return fmt.Errorf("send running: %w", err)
	}

	out, runErr := s.launcher.Run(ctx, stream.RunInput{
		ExperimentID: experiment.Id,
		BacktestName: req.GetBacktestName(),
		ServiceImage: version.ServiceImage,
	})
	if out != nil && out.ExecutionID != "" {
		if err := experiments.SetExecutionID(ctx, experiment.Id, out.ExecutionID); err != nil {
			failErr := fmt.Errorf("set experiment execution id: %w", err)
			_ = experiments.UpdateStatus(ctx, experiment.Id, pb.Experiment_FAILED, failErr)
			return status.Error(codes.Internal, failErr.Error())
		}
	}
	if runErr != nil {
		_ = experiments.UpdateStatus(ctx, experiment.Id, pb.Experiment_FAILED, runErr)
		rsp.RUNNING.State = common.State_STATE_FAILED
		rsp.RUNNING.Message = runErr.Error()
		experiment, _ = experiments.Get(ctx, experiment.Id)
		rsp.Experiment = experiment
		_ = serverStream.Send(rsp)
		return status.Errorf(codes.Internal, "run experiment: %v", runErr)
	}

	if err := experiments.UpdateStatus(ctx, experiment.Id, pb.Experiment_COMPLETED, nil); err != nil {
		return status.Errorf(codes.Internal, "complete experiment: %v", err)
	}
	experiment, err = experiments.Get(ctx, experiment.Id)
	if err != nil {
		return status.Errorf(codes.Internal, "reload experiment: %v", err)
	}

	rsp.RUNNING.State = common.State_STATE_COMPLETED
	rsp.COMPLETED.State = common.State_STATE_COMPLETED
	rsp.Experiment = experiment
	if err := serverStream.Send(rsp); err != nil {
		return fmt.Errorf("send completed: %w", err)
	}
	return nil
}

func (s *StrategyServer) GetExperiment(ctx context.Context, req *servicePb.GetExperimentRequest) (*servicePb.GetExperimentResponse, error) {
	if s.pgx == nil {
		return nil, status.Error(codes.Internal, "strategy storage is not configured")
	}

	experiments := repository.Experiment{Conn: s.pgx}
	experiment, err := experiments.Get(ctx, req.GetExperimentId())
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, status.Errorf(codes.NotFound, "experiment %q not found", req.GetExperimentId())
		}
		return nil, status.Errorf(codes.Internal, "get experiment: %v", err)
	}

	rsp := &servicePb.GetExperimentResponse{Experiment: experiment}
	if experiment.GetExecutionId() != "" {
		path := GrafanaExecutionURL(environment.GetGrafanaURL(), experiment.GetExecutionId())
		rsp.GrafanaPath = &path
		lastPeriod, err := experiments.GetLastPeriod(ctx, experiment.GetExecutionId())
		if err != nil {
			return nil, status.Errorf(codes.Internal, "get last period: %v", err)
		}
		rsp.LastPeriod = lastPeriod
	}
	return rsp, nil
}

func (s *StrategyServer) ListExperiments(ctx context.Context, req *servicePb.ListExperimentsRequest) (*servicePb.ListExperimentsResponse, error) {
	if s.pgx == nil {
		return nil, status.Error(codes.Internal, "strategy storage is not configured")
	}

	experiments := repository.Experiment{Conn: s.pgx}
	list, err := experiments.List(ctx, req.GetStrategyName(), req.GetVersionId(), req.GetBacktestName())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list experiments: %v", err)
	}
	return &servicePb.ListExperimentsResponse{Experiments: list}, nil
}

func (s *StrategyServer) resolveVersion(ctx context.Context, strategyName, versionID string, versionRev int32) (*pb.Version, error) {
	strategies := repository.Strategy{Conn: s.pgx}
	if _, err := strategies.Get(ctx, strategyName); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, status.Errorf(codes.NotFound, "strategy %q not found", strategyName)
		}
		return nil, status.Errorf(codes.Internal, "get strategy: %v", err)
	}

	versions := repository.Version{Conn: s.pgx}
	switch {
	case versionID != "":
		version, err := versions.Get(ctx, versionID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, status.Errorf(codes.NotFound, "version %q not found", versionID)
			}
			return nil, status.Errorf(codes.Internal, "get version: %v", err)
		}
		if version.StrategyName != strategyName {
			return nil, status.Errorf(codes.InvalidArgument, "version %q does not belong to strategy %q",
				versionID, strategyName)
		}
		return version, nil
	case versionRev != 0:
		version, err := versions.GetByStrategyAndRev(ctx, strategyName, versionRev)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, status.Errorf(codes.NotFound, "strategy %q rev %d not found",
					strategyName, versionRev)
			}
			return nil, status.Errorf(codes.Internal, "get version: %v", err)
		}
		return version, nil
	default:
		return nil, status.Error(codes.InvalidArgument, "version_id or version_rev is required")
	}
}

func (s *StrategyServer) requireServiceReady(ctx context.Context, image string) error {
	var serviceStatus int
	err := s.pgx.QueryRow(ctx, `SELECT status FROM service WHERE image=$1`, image).Scan(&serviceStatus)
	if errors.Is(err, pgx.ErrNoRows) {
		return status.Errorf(codes.FailedPrecondition, "service image %q not found", image)
	}
	if err != nil {
		return status.Errorf(codes.Internal, "get service: %v", err)
	}
	st := service.Service_Status_Status(serviceStatus)
	if st != service.Service_Status_READY {
		return status.Errorf(codes.FailedPrecondition,
			"service image %q is not READY (status=%s); interview the service first", image, st)
	}
	return nil
}

func (s *StrategyServer) requireBacktest(ctx context.Context, name string) error {
	var found string
	err := s.pgx.QueryRow(ctx, `SELECT name FROM backtest WHERE name=$1`, name).Scan(&found)
	if errors.Is(err, pgx.ErrNoRows) {
		return status.Errorf(codes.NotFound,
			"backtest %q not found; create and ingest it with `fbull backtest create` — experiments do not auto-ingest",
			name)
	}
	if err != nil {
		return status.Errorf(codes.Internal, "get backtest: %v", err)
	}
	return nil
}
