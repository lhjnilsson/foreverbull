package backtest

import (
	"context"
	"fmt"

	"github.com/lhjnilsson/foreverbull/internal/postgres"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/engine"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/repository"
	backtest_pb "github.com/lhjnilsson/foreverbull/pkg/backtest/pb"
	"github.com/lhjnilsson/foreverbull/pkg/service/worker"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
)

type grpcSessionServer struct {
	backtest_pb.UnimplementedSessionServicerServer

	session *backtest_pb.Session

	db       postgres.Query
	backtest engine.Engine
	wp       worker.Pool
	server   *grpc.Server

	activity chan bool
}

const (
	ActivityBufferSize = 5
)

func NewGRPCSessionServer(session *backtest_pb.Session, database postgres.Query,
	backtest engine.Engine,
) (*grpc.Server, <-chan bool, error) {
	grpcServer := grpc.NewServer()

	activity := make(chan bool, ActivityBufferSize)
	server := &grpcSessionServer{
		session:  session,
		db:       database,
		backtest: backtest,
		server:   grpcServer,
		activity: activity,
	}
	backtest_pb.RegisterSessionServicerServer(grpcServer, server)

	return grpcServer, activity, nil
}

func (s *grpcSessionServer) CreateExecution(ctx context.Context, req *backtest_pb.CreateExecutionRequest) (*backtest_pb.CreateExecutionResponse, error) {
	log.Debug().Msg("create execution")
	select {
	case s.activity <- true:
	default:
	}

	executions := repository.Execution{Conn: s.db}

	execution, err := executions.Create(context.TODO(),
		s.session.Id,
		req.Backtest.StartDate,
		req.Backtest.EndDate,
		req.Backtest.Symbols,
		req.Backtest.Benchmark,
	)
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

	/*
		exf := make([]*service_pb.ExecutionConfiguration_Function, 0)
		for name, parameters := range functions {
			fp := make([]*service_pb.ExecutionConfiguration_FunctionParameter, 0)
			for k, v := range parameters.Parameters {
				fp = append(fp, &service_pb.ExecutionConfiguration_FunctionParameter{
					Key:   k,
					Value: v,
				})
			}
			exf = append(exf, &service_pb.ExecutionConfiguration_Function{
				Name:       name,
				Parameters: fp,
			})
		}
	*/
	log.Debug().Any("execution", execution).Any("configuration", configuration).Msg("execution created")

	return &backtest_pb.CreateExecutionResponse{
		Execution:     execution,
		Configuration: configuration,
	}, nil
}

func (s *grpcSessionServer) RunExecution(req *backtest_pb.RunExecutionRequest, stream backtest_pb.SessionServicer_RunExecutionServer) error {
	log.Debug().Any("request", req).Msg("run execution")
	select {
	case s.activity <- true:
	default:
	}

	executions := repository.Execution{Conn: s.db}

	execution, err := executions.Get(context.Background(), req.ExecutionId)
	if err != nil {
		log.Error().Err(err).Str("execution_id", req.ExecutionId).Msg("error getting execution")
		return fmt.Errorf("error getting execution: %w", err)
	}

	backtest := backtest_pb.Backtest{
		StartDate: execution.StartDate,
		EndDate:   execution.EndDate,
		Symbols:   execution.Symbols,
		Benchmark: execution.Benchmark,
	}

	portfolioCh, err := s.backtest.RunBacktest(context.Background(), &backtest, s.wp)
	if err != nil {
		log.Error().Err(err).Msg("error running backtest")
		return fmt.Errorf("error running backtest: %w", err)
	}

	for portfolio := range portfolioCh {
		err := stream.Send(&backtest_pb.RunExecutionResponse{
			Portfolio: portfolio,
		})
		if err != nil {
			return fmt.Errorf("error sending portfolio: %w", err)
		}
	}

	return nil
}

func (s *grpcSessionServer) GetExecution(ctx context.Context, req *backtest_pb.GetExecutionRequest) (*backtest_pb.GetExecutionResponse, error) {
	log.Debug().Any("request", req).Msg("get execution")
	select {
	case s.activity <- true:
	default:
	}

	executions := repository.Execution{Conn: s.db}

	execution, err := executions.Get(ctx, req.ExecutionId)
	if err != nil {
		return nil, fmt.Errorf("error getting execution: %w", err)
	}

	return &backtest_pb.GetExecutionResponse{
		Execution: execution,
		Periods:   []*backtest_pb.Period{},
	}, nil
}

func (s *grpcSessionServer) StopServer(ctx context.Context, req *backtest_pb.StopServerRequest) (*backtest_pb.StopServerResponse, error) {
	log.Debug().Any("request", req).Msg("stop server")
	if s.wp != nil {
		if err := s.wp.Close(); err != nil {
			log.Error().Err(err).Msg("error closing worker pool")
		}
	}
	close(s.activity)
	return &backtest_pb.StopServerResponse{}, nil
}
