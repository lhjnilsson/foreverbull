package api

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/internal/usercopy"
	"github.com/lhjnilsson/foreverbull/pkg/finance"
	pb "github.com/lhjnilsson/foreverbull/pkg/pb/strategy"
	servicePb "github.com/lhjnilsson/foreverbull/pkg/pb/strategy/service"
	"github.com/lhjnilsson/foreverbull/pkg/strategy/internal/repository"
	"github.com/lhjnilsson/foreverbull/pkg/strategy/internal/runner"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const defaultCalendar = "XNYS"

func (s *StrategyServer) Deploy(ctx context.Context, req *servicePb.DeployRequest) (*servicePb.DeployResponse, error) {
	if s.pgx == nil {
		return nil, status.Error(codes.Internal, "strategy storage is not configured")
	}
	if environment.GetAuthEnabled() {
		return s.deployAuthed(ctx, req)
	}

	if req.GetMode() == pb.Deployment_LIVE {
		return nil, status.Error(codes.FailedPrecondition,
			"live mode is not enabled; deploy with paper")
	}

	dryRun := true
	if req.DryRun != nil {
		dryRun = *req.DryRun
	}
	skipMissed := false
	if req.SkipMissed != nil {
		skipMissed = *req.SkipMissed
	}
	backend := strings.TrimSpace(req.GetBackend())
	if dryRun && backend != "" {
		if err := s.knownProfile(backend); err != nil {
			return nil, err
		}
	}
	if !dryRun {
		if !req.GetConfirm() {
			return nil, status.Error(codes.FailedPrecondition,
				"confirm is required when dry_run=false")
		}
		if !environment.GetTradingEnabled() {
			return nil, status.Error(codes.FailedPrecondition,
				"paper PlaceOrder requires TRADING_ENABLED and a configured Trading backend")
		}
		if backend == "" || s.profiles == nil {
			return nil, status.Error(codes.FailedPrecondition,
				"paper PlaceOrder requires a trading profile name")
		}
		if _, ok := s.profiles.Get(backend); !ok {
			return nil, status.Errorf(codes.FailedPrecondition,
				"unknown trading profile %q", backend)
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

	calendar := req.GetCalendarCode()
	if calendar == "" {
		calendar = defaultCalendar
	}
	if err := s.refuseDeployMarkets(ctx, req.GetStrategyName(), backend, req.GetSymbols()); err != nil {
		return nil, err
	}

	deployments := repository.Deployment{Conn: s.pgx}
	deployment, err := deployments.Create(
		ctx, version.Id, backend, calendar,
		pb.Deployment_PAPER, req.GetSymbols(), pb.Deployment_SESSION_CLOSE, dryRun, skipMissed,
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
			failErr := deployments.Fail(ctx, deployment.Id, err)
			notStored := repository.StatusNotStored(failErr)
			log.Error().Err(failErr).Str("deployment", deployment.Id).Bool("error_not_stored", notStored).
				Msg("error starting deployment runner")
			return nil, admissionStatus(req.GetStrategyName(), err, notStored)
		}
	}

	return &servicePb.DeployResponse{Deployment: deployment}, nil
}

// admissionStatus is the deploy error after Add failed and Fail stored it.
// An admission sentence is shown as itself. Any other start failure keeps
// the generic sentence. notStored means the ERROR row could not be written.
func admissionStatus(strategy string, err error, notStored bool) error {
	var admitted *runner.AdmissionError
	if !notStored && errors.As(err, &admitted) {
		return status.Error(codes.FailedPrecondition, admitted.Error())
	}
	return status.Error(codes.Internal, usercopy.DeploymentStartFailed(strategy, notStored))
}

// refuseDeployMarkets stops a deployment whose symbols are listed outside
// the US or priced in a currency other than USD. The listing source is the
// profile's provider: Alpaca for an Alpaca profile, Yahoo otherwise. It has
// no benchmark. The check runs before the deployment row, the runner, and
// any order. A symbol the source does not know is not a pass. A non-US
// suffix refuses without a later ticker or a second lookup.
func (s *StrategyServer) refuseDeployMarkets(ctx context.Context, name, profile string, symbols []string) error {
	provider, err := finance.ListingProvider(profile)
	if err != nil {
		return status.Error(codes.FailedPrecondition, usercopy.DeploymentProfileUnchecked(name, strings.TrimSpace(profile), !errors.Is(err, finance.ErrProfileNotFound)))
	}
	listed, _, err := finance.RefuseListed(ctx, provider, profile, symbols, "")
	names, lookupProvider, lookup := deployLookups(err)
	if lookupProvider == "" {
		lookupProvider = provider
	}
	if !lookup && err != nil {
		return status.Error(codes.FailedPrecondition, err.Error())
	}
	switch {
	case len(names) > 0 && len(listed) > 0:
		return status.Error(codes.FailedPrecondition, usercopy.GlobalMarketsDeployMixed(name, listed, lookupProvider, names))
	case len(names) > 0:
		return status.Error(codes.FailedPrecondition, usercopy.LookupDeploy(name, lookupProvider, names))
	case len(listed) > 0:
		return status.Error(codes.FailedPrecondition, usercopy.GlobalMarketsDeploy(name, listed))
	}
	if !finance.ListingsAnswered(provider, symbols) {
		return status.Error(codes.FailedPrecondition, usercopy.DeploymentSymbolsUnknown(name, provider, finance.Unanswered(provider, symbols)))
	}
	return nil
}

func deployLookups(err error) (names []string, provider string, ok bool) {
	var many *finance.QuoteFailures
	if errors.As(err, &many) && many != nil {
		for _, item := range many.Items {
			if item == nil {
				continue
			}
			names = append(names, item.Symbol)
			if provider == "" {
				provider = item.Provider
			}
		}
		return names, provider, true
	}
	var one *finance.QuoteFailure
	if errors.As(err, &one) && one != nil {
		return []string{one.Symbol}, one.Provider, true
	}
	return nil, "", false
}

// knownProfile refuses a trading profile name that is not in the registry.
// A nil registry and an unknown name are different sentences. Both are
// FailedPrecondition and neither starts a deployment.
func (s *StrategyServer) knownProfile(name string) error {
	if s.profiles == nil {
		return status.Error(codes.FailedPrecondition, usercopy.NoTradingProfiles(name))
	}
	if _, ok := s.profiles.Get(name); !ok {
		return status.Error(codes.FailedPrecondition, usercopy.UnknownTradingProfile(name))
	}
	return nil
}

func (s *StrategyServer) StopDeployment(ctx context.Context, req *servicePb.StopDeploymentRequest) (*servicePb.StopDeploymentResponse, error) {
	if s.pgx == nil {
		return nil, status.Error(codes.Internal, "strategy storage is not configured")
	}
	if environment.GetAuthEnabled() {
		return s.stopDeploymentAuthed(ctx, req)
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
		// Remove waits for the loop to end before the row reads STOPPED, so
		// nothing the loop writes lands after STOPPED. Both run detached from
		// the request and bounded, so a client that leaves can't cut them off.
		if s.runner != nil {
			removeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), repository.StatusWriteTimeout)
			err := s.runner.Remove(removeCtx, deployment.Id)
			cancel()
			if err != nil {
				log.Error().Err(err).Str("deployment", deployment.Id).Msg("error stopping deployment runner")
				return nil, status.Error(codes.Internal, usercopy.DeploymentStopFailed(deployment.GetStrategyName()))
			}
		}
		stopCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), repository.StatusWriteTimeout)
		defer cancel()
		if err := deployments.Stop(stopCtx, deployment.Id); err != nil {
			return nil, status.Errorf(codes.Internal, "stop deployment: %v", err)
		}
		deployment, err = deployments.Get(stopCtx, deployment.Id)
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
	if environment.GetAuthEnabled() {
		return s.listDeploymentRunsAuthed(ctx, req)
	}

	runs := repository.DeploymentRun{Conn: s.pgx}
	list, err := runs.List(ctx, req.GetStrategyName(), req.GetDeploymentId())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list deployment runs: %v", err)
	}
	return &servicePb.ListDeploymentRunsResponse{Runs: list}, nil
}

// ListDeployments returns every deployment of the named strategy, oldest
// started_at first. It is not filtered by a deployment id.
func (s *StrategyServer) ListDeployments(ctx context.Context, req *servicePb.ListDeploymentsRequest) (*servicePb.ListDeploymentsResponse, error) {
	if s.pgx == nil {
		return nil, status.Error(codes.Internal, "strategy storage is not configured")
	}
	if environment.GetAuthEnabled() {
		return s.listDeploymentsAuthed(ctx, req)
	}
	store := repository.Deployment{Conn: s.pgx}
	deployments, err := store.List(ctx, req.GetStrategyName())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list deployments: %v", err)
	}
	return &servicePb.ListDeploymentsResponse{Deployments: deployments}, nil
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
