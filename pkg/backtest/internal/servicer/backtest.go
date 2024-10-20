package servicer

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/stream"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/repository"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/pb"
	msg "github.com/lhjnilsson/foreverbull/pkg/backtest/stream"
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
	req *pb.ListBacktestsRequest) (*pb.ListBacktestsResponse, error) {
	backtests := repository.Backtest{Conn: bs.pgx}

	list, err := backtests.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("error listing backtests: %w", err)
	}

	return &pb.ListBacktestsResponse{
		Backtests: list,
	}, nil
}

func (bs *BacktestServer) CreateBacktest(ctx context.Context,
	req *pb.CreateBacktestRequest) (*pb.CreateBacktestResponse, error) {
	backtests := repository.Backtest{Conn: bs.pgx}

	reqBacktest := req.GetBacktest()
	if reqBacktest == nil {
		return nil, fmt.Errorf("backtest is required")
	}

	backtest, err := backtests.Create(ctx, reqBacktest.GetName(), reqBacktest.StartDate,
		reqBacktest.EndDate, reqBacktest.Symbols, reqBacktest.Benchmark)
	if err != nil {
		return nil, fmt.Errorf("error creating backtest: %w", err)
	}

	return &pb.CreateBacktestResponse{
		Backtest: backtest,
	}, nil
}

func (bs *BacktestServer) GetBacktest(ctx context.Context,
	req *pb.GetBacktestRequest) (*pb.GetBacktestResponse, error) {
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
	req *pb.CreateSessionRequest) (*pb.CreateSessionResponse, error) {
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
	req *pb.GetSessionRequest) (*pb.GetSessionResponse, error) {
	sessions := repository.Session{Conn: bs.pgx}

	session, err := sessions.Get(ctx, req.GetSessionId())
	if err != nil {
		return nil, fmt.Errorf("error getting session: %w", err)
	}

	return &pb.GetSessionResponse{
		Session: session,
	}, nil
}
