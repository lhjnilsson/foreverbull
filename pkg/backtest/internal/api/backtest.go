package api

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/stream"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/repository"
	backtestStream "github.com/lhjnilsson/foreverbull/pkg/backtest/stream"
	commonPb "github.com/lhjnilsson/foreverbull/pkg/pb"
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
	_stream servicePb.BacktestServicer_CreateBacktestServer,
) error {
	ctx := _stream.Context()
	backtests := repository.Backtest{Conn: bs.pgx}
	rsp := &servicePb.CreateBacktestResponse{
		CREATING: &commonPb.Step{
			State: commonPb.State_STATE_RUNNING,
		},
		DOWNLOADING: &commonPb.Step{
			State: commonPb.State_STATE_UNSPECIFIED,
		},
		INGESTING: &commonPb.Step{
			State: commonPb.State_STATE_UNSPECIFIED,
		},
		STORING: &commonPb.Step{
			State: commonPb.State_STATE_UNSPECIFIED,
		},
		Backtest: nil,
	}
	if err := _stream.Send(rsp); err != nil {
		return fmt.Errorf("error sending response: %w", err)
	}

	backtest, err := backtests.Create(ctx, req.GetBacktest())
	if err != nil {
		return fmt.Errorf("error creating backtest: %w", err)
	}
	rsp.Backtest = backtest
	rsp.CREATING.State = commonPb.State_STATE_COMPLETED
	if err := _stream.Send(rsp); err != nil {
		return fmt.Errorf("error sending response: %w", err)
	}

	o, err := backtestStream.NewCreateBacktestOrchestration(req.GetBacktest().GetName(),
		[]string{}, "", "")
	if err != nil {
		return fmt.Errorf("error creating backtest orchestration: %w", err)
	}

	steps := bs.stream.RunOrchestration(ctx, o)
	for step := range steps {
		err = stream.UpdateOrchestration(rsp, step)
		if err != nil {
			return fmt.Errorf("error updating orchestration: %w", err)
		}
		err = _stream.Send(rsp)
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
	rsp := &servicePb.CreateSessionResponse{
		CREATING: &commonPb.Step{
			State: commonPb.State_STATE_RUNNING,
		},
		STARTING: &commonPb.Step{
			State: commonPb.State_STATE_UNSPECIFIED,
		},
		STORING: &commonPb.Step{
			State: commonPb.State_STATE_UNSPECIFIED,
		},
		Session: nil,
	}
	if err := _stream.Send(rsp); err != nil {
		return fmt.Errorf("error sending response: %w", err)
	}

	session, err := sessions.Create(ctx, req.GetBacktestName())
	if err != nil {
		return fmt.Errorf("error creating session: %w", err)
	}
	rsp.Session = session
	rsp.CREATING.State = commonPb.State_STATE_COMPLETED
	if err := _stream.Send(rsp); err != nil {
		return fmt.Errorf("error sending response: %w", err)
	}

	msg, err := backtestStream.NewSessionRunCommand(session.Backtest, session.Id)
	if err != nil {
		return fmt.Errorf("error creating session run command: %w", err)
	}
	rsp.STARTING.State = commonPb.State_STATE_RUNNING
	if err := _stream.Send(rsp); err != nil {
		return fmt.Errorf("error sending response: %w", err)
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

	rsp.STARTING.State = commonPb.State_STATE_COMPLETED
	rsp.Session = session
	if err := _stream.Send(rsp); err != nil {
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
