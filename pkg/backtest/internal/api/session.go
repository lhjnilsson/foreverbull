package api

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/lhjnilsson/foreverbull/internal/auth"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/internal/postgres"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/engine"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/repository"
	"github.com/lhjnilsson/foreverbull/pkg/pb/backtest"
	servicePb "github.com/lhjnilsson/foreverbull/pkg/pb/backtest/service"
	"github.com/lhjnilsson/foreverbull/pkg/service/worker"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type grpcSessionServer struct {
	servicePb.UnimplementedSessionServicerServer

	session *backtest.Session

	db              postgres.Query
	backtestSession engine.EngineSession
	wp              worker.Pool

	activity chan bool
}

const (
	ActivityBufferSize = 5
)

func NewGRPCSessionServer(session *backtest.Session, database postgres.Query,
	ziplineEngine engine.Engine,
) (*grpc.Server, <-chan bool, error) {
	grpcServer := grpc.NewServer()

	backtestSession, err := ziplineEngine.NewSession(context.Background(), session)
	if err != nil {
		log.Error().Err(err).Msg("fail to create zipline session")
		return nil, nil, fmt.Errorf("error creating session: %w", err)
	}

	activity := make(chan bool, ActivityBufferSize)
	server := &grpcSessionServer{
		session:         session,
		db:              database,
		backtestSession: backtestSession,
		activity:        activity,
	}
	servicePb.RegisterSessionServicerServer(grpcServer, server)

	return grpcServer, activity, nil
}

// NewSecuredGRPCSessionServer requires x-foreverbull-session on every call.
// The secret is the value minted for this session. It is not the user bearer.
func NewSecuredGRPCSessionServer(session *backtest.Session, database postgres.Query,
	ziplineEngine engine.Engine, secret string,
) (*grpc.Server, <-chan bool, error) {
	if secret == "" {
		return nil, nil, fmt.Errorf("%s is missing", auth.SessionMetadata())
	}
	grpcServer := grpc.NewServer(auth.SessionInterceptors(secret)...)

	backtestSession, err := ziplineEngine.NewSession(context.Background(), session)
	if err != nil {
		log.Error().Err(err).Msg("fail to create zipline session")
		return nil, nil, fmt.Errorf("error creating session: %w", err)
	}

	activity := make(chan bool, ActivityBufferSize)
	server := &grpcSessionServer{
		session:         session,
		db:              database,
		backtestSession: backtestSession,
		activity:        activity,
	}
	servicePb.RegisterSessionServicerServer(grpcServer, server)
	return grpcServer, activity, nil
}

// storedBacktest loads the row the session was opened on. Auth-on sessions
// store the backtest id. Auth-off sessions store the name.
func (s *grpcSessionServer) storedBacktest(ctx context.Context) (*backtest.Backtest, error) {
	backtests := repository.Backtest{Conn: s.db}
	if !environment.GetAuthEnabled() {
		return backtests.Get(ctx, s.session.GetBacktest())
	}
	sessions := repository.Session{Conn: s.db}
	id, err := sessions.BacktestID(ctx, s.session.GetId())
	if err != nil {
		return nil, err
	}
	return backtests.GetByID(ctx, id)
}

func backtestLoadStatus(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return status.Error(codes.NotFound, "backtest not found")
	}
	return status.Error(codes.Internal, "could not load the backtest")
}

func (s *grpcSessionServer) pingActivity() {
	select {
	case s.activity <- true:
	default:
	}
}

func (s *grpcSessionServer) CreateExecution(ctx context.Context, req *servicePb.CreateExecutionRequest) (*servicePb.CreateExecutionResponse, error) {
	log.Debug().Msg("create execution")
	s.pingActivity()

	if environment.GetAuthEnabled() && req.GetSession() != s.session.GetId() {
		return nil, status.Error(codes.NotFound, "not found")
	}
	stored, err := s.storedBacktest(ctx)
	if err != nil {
		log.Error().Err(err).Str("session", s.session.GetId()).Msg("error getting backtest")
		return nil, backtestLoadStatus(err)
	}
	startDate, endDate, err := resolveRunDates(stored, req.GetStartDate(), req.GetEndDate())
	if err != nil {
		log.Info().Err(err).Str("backtest", s.session.GetBacktest()).Msg("execution dates rejected")
		return nil, err
	}

	executions := repository.Execution{Conn: s.db}
	e := backtest.Execution{
		Session:   s.session.Id,
		StartDate: startDate,
		EndDate:   endDate,
	}

	execution, err := executions.Create(ctx, &e, executionFunctionsFromAlgorithm(req.GetAlgorithm()))
	if err != nil {
		log.Error().Err(err).Msg("error creating execution")
		return nil, fmt.Errorf("error creating execution: %w", err)
	}

	s.wp, err = worker.NewPool(ctx, req.GetAlgorithm())
	if err != nil {
		log.Error().Err(err).Msg("error creating worker pool")
		return nil, fmt.Errorf("error creating worker pool: %w", err)
	}

	configuration := s.wp.Configure()
	log.Debug().Str("execution_id", execution.GetId()).Msg("execution created")

	return &servicePb.CreateExecutionResponse{
		Execution:     execution,
		Configuration: configuration,
	}, nil
}

func (s *grpcSessionServer) RunExecution(req *servicePb.RunExecutionRequest, stream servicePb.SessionServicer_RunExecutionServer) error {
	log.Debug().Any("request", req).Msg("run execution")
	s.pingActivity()
	ctx := stream.Context()

	execution, err := s.executionForListener(ctx, req.ExecutionId)
	if err != nil {
		log.Error().Err(err).Str("execution_id", req.ExecutionId).Msg("error getting execution")
		return err
	}

	periods, err := s.backtestSession.RunBacktest(ctx, execution, s.wp)
	if err != nil {
		log.Error().Err(err).Msg("error running backtest")
		return fmt.Errorf("error running backtest: %w", err)
	}

	for period := range periods {
		err := stream.Send(period)
		if err != nil {
			return fmt.Errorf("error sending portfolio: %w", err)
		}
	}
	return nil
}

func (s *grpcSessionServer) StoreResult(ctx context.Context, req *servicePb.StoreExecutionResultRequest) (*servicePb.StoreExecutionResultResponse, error) {
	log.Debug().Any("request", req).Msg("store result")
	if _, err := s.executionForListener(ctx, req.ExecutionId); err != nil {
		return nil, err
	}
	rsp, err := s.backtestSession.GetResult(ctx, req.ExecutionId)
	if err != nil {
		return nil, fmt.Errorf("error getting result: %w", err)
	}
	executions := repository.Execution{Conn: s.db}
	err = executions.StorePeriods(ctx, req.ExecutionId, rsp.Periods)
	if err != nil {
		return nil, fmt.Errorf("error storing result: %w", err)
	}
	return &servicePb.StoreExecutionResultResponse{}, nil
}

func (s *grpcSessionServer) executionForListener(ctx context.Context, executionID string) (*backtest.Execution, error) {
	executions := repository.Execution{Conn: s.db}
	execution, err := executions.Get(ctx, executionID)
	if err != nil {
		if environment.GetAuthEnabled() {
			return nil, status.Error(codes.NotFound, "not found")
		}
		return nil, fmt.Errorf("error getting execution: %w", err)
	}
	if environment.GetAuthEnabled() && execution.GetSession() != s.session.GetId() {
		return nil, status.Error(codes.NotFound, "not found")
	}
	return execution, nil
}

func (s *grpcSessionServer) StopServer(_ context.Context, _ *servicePb.StopServerRequest) (*servicePb.StopServerResponse, error) {
	log.Debug().Msg("stop server")
	if s.wp != nil {
		if err := s.wp.Close(); err != nil {
			log.Error().Err(err).Msg("error closing worker pool")
		}
	}
	close(s.activity)
	return &servicePb.StopServerResponse{}, nil
}
