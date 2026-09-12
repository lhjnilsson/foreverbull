package api

import (
	"context"
	"fmt"

	"github.com/lhjnilsson/foreverbull/internal/postgres"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/engine"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/repository"
	"github.com/lhjnilsson/foreverbull/pkg/pb/backtest"
	servicePb "github.com/lhjnilsson/foreverbull/pkg/pb/backtest/service"
	"github.com/lhjnilsson/foreverbull/pkg/service/worker"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
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

func (s *grpcSessionServer) pingActivity() {
	select {
	case s.activity <- true:
	default:
	}
}

func (s *grpcSessionServer) CreateExecution(ctx context.Context, req *servicePb.CreateExecutionRequest) (*servicePb.CreateExecutionResponse, error) {
	log.Debug().Msg("create execution")
	s.pingActivity()

	executions := repository.Execution{Conn: s.db}
	e := backtest.Execution{
		Session:   s.session.Id,
		StartDate: req.StartDate,
		EndDate:   req.EndDate,
	}

	execution, err := executions.Create(ctx, &e)
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
	log.Debug().Any("execution", execution).Any("configuration", configuration).Msg("execution created")

	return &servicePb.CreateExecutionResponse{
		Execution:     execution,
		Configuration: configuration,
	}, nil
}

func (s *grpcSessionServer) RunExecution(req *servicePb.RunExecutionRequest, stream servicePb.SessionServicer_RunExecutionServer) error {
	log.Debug().Any("request", req).Msg("run execution")
	s.pingActivity()
	ctx := stream.Context()
	executions := repository.Execution{Conn: s.db}

	execution, err := executions.Get(ctx, req.ExecutionId)
	if err != nil {
		log.Error().Err(err).Str("execution_id", req.ExecutionId).Msg("error getting execution")
		return fmt.Errorf("error getting execution: %w", err)
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

func (s *grpcSessionServer) GetExecution(ctx context.Context, req *servicePb.GetExecutionRequest) (*servicePb.GetExecutionResponse, error) {
	log.Debug().Any("request", req).Msg("get execution")
	s.pingActivity()

	executions := repository.Execution{Conn: s.db}

	execution, err := executions.Get(ctx, req.ExecutionId)
	if err != nil {
		return nil, fmt.Errorf("error getting execution: %w", err)
	}

	return &servicePb.GetExecutionResponse{
		Execution: execution,
		Periods:   []*backtest.Period{},
	}, nil
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
