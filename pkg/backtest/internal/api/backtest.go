package api

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/stream"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/repository"
	backtestStream "github.com/lhjnilsson/foreverbull/pkg/backtest/stream"
	msg "github.com/lhjnilsson/foreverbull/pkg/backtest/stream"
	pbInternal "github.com/lhjnilsson/foreverbull/pkg/pb"
	servicePb "github.com/lhjnilsson/foreverbull/pkg/pb/backtest/service"
)

type BacktestServer struct {
	servicePb.UnimplementedBacktestServicerServer

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
	req *servicePb.ListBacktestsRequest,
) (*servicePb.ListBacktestsResponse, error) {
	backtests := repository.Backtest{Conn: bs.pgx}

	list, err := backtests.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("error listing backtests: %w", err)
	}

	return &servicePb.ListBacktestsResponse{
		Backtests: list,
	}, nil
}

func (bs *BacktestServer) CreateBacktest(req *servicePb.CreateBacktestRequest,
	stream servicePb.BacktestServicer_CreateBacktestServer,
) error {
	ctx := stream.Context()
	backtests := repository.Backtest{Conn: bs.pgx}
	backtest, err := backtests.Create(ctx, req.GetBacktest().GetName(), req.GetBacktest().StartDate,
		req.GetBacktest().EndDate, req.GetBacktest().Symbols, req.GetBacktest().Benchmark)
	if err != nil {
		return fmt.Errorf("error creating backtest: %w", err)
	}

	o, err := backtestStream.NewCreateBacktestOrchestration(req.GetBacktest().GetName(),
		req.GetBacktest().GetSymbols(), pbInternal.DateToDateString(req.GetBacktest().GetStartDate()),
		pbInternal.DateToDateString(req.GetBacktest().GetEndDate()),
	)
	if err != nil {
		return fmt.Errorf("error creating backtest orchestration: %w", err)
	}

	steps := bs.stream.RunOrchestration(ctx, o)
	for step := range steps {
		err = stream.Send(
			&servicePb.CreateBacktestResponse{
				Backtest: backtest,
				Status: &servicePb.CreateBacktestResponse_Status{
					Step:  servicePb.CreateBacktestResponse_Step(step.Num),
					State: step.State,
				},
			})
		if err != nil {
			return fmt.Errorf("error sending response: %w", err)
		}
	}
	return nil
}

func (bs *BacktestServer) GetBacktest(ctx context.Context,
	req *servicePb.GetBacktestRequest,
) (*servicePb.GetBacktestResponse, error) {
	backtests := repository.Backtest{Conn: bs.pgx}

	backtest, err := backtests.Get(ctx, req.GetName())
	if err != nil {
		return nil, fmt.Errorf("error getting backtest: %w", err)
	}

	return &servicePb.GetBacktestResponse{
		Name:     backtest.Name,
		Backtest: backtest,
	}, nil
}

func (bs *BacktestServer) CreateSession(
	req *servicePb.CreateSessionRequest, _stream servicePb.BacktestServicer_CreateSessionServer,
) error {
	sessions := repository.Session{Conn: bs.pgx}
	ctx := _stream.Context()

	session, err := sessions.Create(ctx, req.GetBacktestName())
	if err != nil {
		return fmt.Errorf("error creating session: %w", err)
	}
	err = _stream.Send(&servicePb.CreateSessionResponse{
		Session: session,
		Status: &servicePb.CreateSessionResponse_Status{
			Step:  servicePb.CreateSessionResponse_CREATING,
			State: pbInternal.State_STATE_COMPLETED,
		},
	})
	if err != nil {
		return fmt.Errorf("error sending response: %w", err)
	}

	msg, err := msg.NewSessionRunCommand(session.Backtest, session.Id)
	if err != nil {
		return fmt.Errorf("error creating session run command: %w", err)
	}

	err = bs.stream.Publish(ctx, msg, stream.WaitForCompletion())
	if err != nil {
		return fmt.Errorf("error publishing session run command: %w", err)
	}

	// Refetch session to get port
	session, err = sessions.Get(ctx, session.Id)
	if err != nil {
		return fmt.Errorf("error getting session: %w", err)
	}

	err = _stream.Send(&servicePb.CreateSessionResponse{
		Session: session,
		Status: &servicePb.CreateSessionResponse_Status{
			Step:  servicePb.CreateSessionResponse_STARTING,
			State: pbInternal.State_STATE_COMPLETED,
		},
	})
	if err != nil {
		return fmt.Errorf("error sending response: %w", err)
	}
	return nil
}

func (bs *BacktestServer) GetSession(ctx context.Context,
	req *servicePb.GetSessionRequest,
) (*servicePb.GetSessionResponse, error) {
	sessions := repository.Session{Conn: bs.pgx}

	session, err := sessions.Get(ctx, req.GetSessionId())
	if err != nil {
		return nil, fmt.Errorf("error getting session: %w", err)
	}

	return &servicePb.GetSessionResponse{
		Session: session,
	}, nil
}

func (bs *BacktestServer) ListExecutions(ctx context.Context,
	req *servicePb.ListExecutionsRequest,
) (*servicePb.ListExecutionsResponse, error) {
	storage := repository.Execution{Conn: bs.pgx}

	if req.GetSessionId() != "" {
		executions, err := storage.ListBySession(ctx, req.GetSessionId())
		if err != nil {
			return nil, fmt.Errorf("error listing executions: %w", err)
		}

		return &servicePb.ListExecutionsResponse{
			Executions: executions,
		}, nil
	} else if req.GetBacktest() != "" {
		executions, err := storage.ListByBacktest(ctx, req.GetBacktest())
		if err != nil {
			return nil, fmt.Errorf("error listing executions: %w", err)
		}

		return &servicePb.ListExecutionsResponse{
			Executions: executions,
		}, nil
	}

	executions, err := storage.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("error listing executions: %w", err)
	}

	return &servicePb.ListExecutionsResponse{
		Executions: executions,
	}, nil
}

func (bs *BacktestServer) GetExecution(ctx context.Context,
	req *servicePb.GetExecutionRequest,
) (*servicePb.GetExecutionResponse, error) {
	storage := repository.Execution{Conn: bs.pgx}

	execution, err := storage.Get(ctx, req.GetExecutionId())
	if err != nil {
		return nil, fmt.Errorf("error getting execution: %w", err)
	}
	periods, err := storage.GetPeriods(ctx, req.GetExecutionId())
	if err != nil {
		return nil, fmt.Errorf("error getting execution periods: %w", err)
	}

	return &servicePb.GetExecutionResponse{
		Execution: execution,
		Periods:   periods,
	}, nil
}
