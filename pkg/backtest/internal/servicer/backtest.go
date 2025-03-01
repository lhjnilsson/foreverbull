package servicer

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/stream"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/repository"
	backtestStream "github.com/lhjnilsson/foreverbull/pkg/backtest/stream"
	msg "github.com/lhjnilsson/foreverbull/pkg/backtest/stream"
	pb "github.com/lhjnilsson/foreverbull/pkg/pb/backtest"
)

type BacktestServer struct {
	pb.UnimplementedBacktestServicerServer

	pgx    *pgxpool.Pool
	stream stream.Stream
}

func NewBacktestServer(pgx *pgxpool.Pool, stream stream.Stream) *BacktestServer {
	return &BacktestServer{
		pgx:    pgx,
		stream: stream,
	}
}

func (bs *BacktestServer) ListBacktests(ctx context.Context,
	req *pb.ListBacktestsRequest,
) (*pb.ListBacktestsResponse, error) {
	backtests := repository.Backtest{Conn: bs.pgx}

	list, err := backtests.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("error listing backtests: %w", err)
	}

	return &pb.ListBacktestsResponse{
		Backtests: list,
	}, nil
}

func (bs *BacktestServer) CreateBacktest(req *pb.CreateBacktestRequest,
	stream pb.BacktestServicer_CreateBacktestServer,
) error {
	ctx := stream.Context()
	backtests := repository.Backtest{Conn: bs.pgx}

	backtest, err := backtests.Create(ctx, req.GetBacktest().GetName(), req.GetBacktest().StartDate,
		req.GetBacktest().EndDate, req.GetBacktest().Symbols, req.GetBacktest().Benchmark)
	if err != nil {
		return fmt.Errorf("error creating backtest: %w", err)
	}

	o, err := backtestStream.NewCreateBacktestOrchestration(req.GetBacktest().GetName(),
		req.GetBacktest().GetSymbols(), req.GetBacktest().StartDate.String(),
		req.GetBacktest().EndDate.String(),
	)
	if err != nil {
		return fmt.Errorf("error creating backtest orchestration: %w", err)
	}
	err = bs.stream.RunOrchestration(ctx, o)
	if err != nil {
		return fmt.Errorf("error running backtest orchestration: %w", err)
	}

	err = stream.Send(
		&pb.CreateBacktestResponse{
			Backtest: backtest,
		},
	)
	if err != nil {
		return fmt.Errorf("error sending backtest response: %w", err)
	}

	lastStatus := backtest.Statuses[0].Status
	for {
		time.Sleep(time.Second / 4)
		backtest, err = backtests.Get(ctx, backtest.Name)
		if err != nil {
			return fmt.Errorf("error getting backtest: %w", err)
		}
		if backtest.Statuses[0].Status == lastStatus {
			continue
		}
		lastStatus = backtest.Statuses[0].Status
		err = stream.Send(&pb.CreateBacktestResponse{
			Backtest: backtest,
		})
		if err != nil {
			return fmt.Errorf("error sending backtest response: %w", err)
		}
		if lastStatus == pb.BacktestStatus_READY {
			break
		}
	}
	return nil
}

func (bs *BacktestServer) GetBacktest(ctx context.Context,
	req *pb.GetBacktestRequest,
) (*pb.GetBacktestResponse, error) {
	backtests := repository.Backtest{Conn: bs.pgx}

	backtest, err := backtests.Get(ctx, req.GetName())
	if err != nil {
		return nil, fmt.Errorf("error getting backtest: %w", err)
	}

	return &pb.GetBacktestResponse{
		Name:     backtest.Name,
		Backtest: backtest,
	}, nil
}

func (bs *BacktestServer) CreateSession(ctx context.Context,
	req *pb.CreateSessionRequest,
) (*pb.CreateSessionResponse, error) {
	sessions := repository.Session{Conn: bs.pgx}

	session, err := sessions.Create(ctx, req.GetBacktestName())
	if err != nil {
		return nil, fmt.Errorf("error creating session: %w", err)
	}

	msg, err := msg.NewSessionRunCommand(session.Backtest, session.Id)
	if err != nil {
		return nil, fmt.Errorf("error creating session run command: %w", err)
	}

	err = bs.stream.Publish(ctx, msg)
	if err != nil {
		return nil, fmt.Errorf("error publishing session run command: %w", err)
	}

	return &pb.CreateSessionResponse{
		Session: session,
	}, nil
}

func (bs *BacktestServer) GetSession(ctx context.Context,
	req *pb.GetSessionRequest,
) (*pb.GetSessionResponse, error) {
	sessions := repository.Session{Conn: bs.pgx}

	session, err := sessions.Get(ctx, req.GetSessionId())
	if err != nil {
		return nil, fmt.Errorf("error getting session: %w", err)
	}

	return &pb.GetSessionResponse{
		Session: session,
	}, nil
}

func (bs *BacktestServer) ListExecutions(ctx context.Context,
	req *pb.ListExecutionsRequest,
) (*pb.ListExecutionsResponse, error) {
	storage := repository.Execution{Conn: bs.pgx}

	if req.GetSessionId() != "" {
		executions, err := storage.ListBySession(ctx, req.GetSessionId())
		if err != nil {
			return nil, fmt.Errorf("error listing executions: %w", err)
		}

		return &pb.ListExecutionsResponse{
			Executions: executions,
		}, nil
	} else if req.GetBacktest() != "" {
		executions, err := storage.ListByBacktest(ctx, req.GetBacktest())
		if err != nil {
			return nil, fmt.Errorf("error listing executions: %w", err)
		}

		return &pb.ListExecutionsResponse{
			Executions: executions,
		}, nil
	}

	executions, err := storage.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("error listing executions: %w", err)
	}

	return &pb.ListExecutionsResponse{
		Executions: executions,
	}, nil
}

func (bs *BacktestServer) GetExecution(ctx context.Context,
	req *pb.GetExecutionRequest,
) (*pb.GetExecutionResponse, error) {
	storage := repository.Execution{Conn: bs.pgx}

	execution, err := storage.Get(ctx, req.GetExecutionId())
	if err != nil {
		return nil, fmt.Errorf("error getting execution: %w", err)
	}
	periods, err := storage.GetPeriods(ctx, req.GetExecutionId())
	if err != nil {
		return nil, fmt.Errorf("error getting execution periods: %w", err)
	}

	return &pb.GetExecutionResponse{
		Execution: execution,
		Periods:   periods,
	}, nil
}
