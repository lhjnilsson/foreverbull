package backtest

import (
	"context"
	"fmt"

	"github.com/lhjnilsson/foreverbull/internal/postgres"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/engine"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/entity"
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

	currentExecution *entity.Execution
}

func NewGRPCSessionServer(session *backtest_pb.Session, db postgres.Query,
	backtest engine.Engine) (*grpc.Server, <-chan bool, error) {
	g := grpc.NewServer()

	activity := make(chan bool, 5)
	server := &grpcSessionServer{
		session:  session,
		db:       db,
		backtest: backtest,
		server:   g,
		activity: activity,
	}
	backtest_pb.RegisterSessionServicerServer(g, server)
	return g, activity, nil
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
		req.Backtest.StartDate.AsTime(),
		req.Backtest.EndDate.AsTime(),
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
		return fmt.Errorf("error getting execution: %w", err)
	}
	backtest := backtest_pb.Backtest{
		StartDate: execution.StartDate,
		EndDate:   execution.EndDate,
		Symbols:   execution.Symbols,
		Benchmark: execution.Benchmark,
	}
	ch, err := s.backtest.RunBacktest(context.Background(), &backtest, s.wp)
	if err != nil {
		return err
	}
	for p := range ch {
		err := stream.Send(&backtest_pb.RunExecutionResponse{
			Portfolio: p,
		})
		if err != nil {
			return err
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
		return nil, err
	}
	return &backtest_pb.GetExecutionResponse{
		Execution: execution,
		Periods:   []*backtest_pb.Period{},
	}, nil
}

func (s *grpcSessionServer) StopServer(ctx context.Context, req *backtest_pb.StopServerRequest) (*backtest_pb.StopServerResponse, error) {
	log.Debug().Any("request", req).Msg("stop server")
	close(s.activity)
	return &backtest_pb.StopServerResponse{}, nil
}
