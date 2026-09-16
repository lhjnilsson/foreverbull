package backtest

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/internal/storage"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/engine"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/repository"
	pb "github.com/lhjnilsson/foreverbull/pkg/pb/backtest"
	servicePb "github.com/lhjnilsson/foreverbull/pkg/pb/backtest/service"
	"github.com/lhjnilsson/foreverbull/pkg/service/worker"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// MemberSymbols maps a backtest's universe_member rows through asset_symbol at t.
type MemberSymbols interface {
	SymbolsAt(ctx context.Context, backtestName string, t time.Time) ([]string, error)
}

func NewZiplineEngineSession(ctx context.Context, connStr string) (engine.EngineSession, error) {
	conn, err := grpc.NewClient(
		connStr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("error getting grpc client: %w", err)
	}

	client := servicePb.NewEngineSessionClient(conn)

	return &ZipelineSession{client: client}, nil
}

type ZipelineSession struct {
	client    servicePb.EngineSessionClient
	symbolsAt MemberSymbols
}

func (z *ZipelineSession) RunBacktest(ctx context.Context, execution *pb.Execution, workers worker.Pool) (chan *servicePb.RunExecutionResponse, error) {
	req := servicePb.RunBacktestRequest{
		Execution: execution,
	}

	rsp, err := z.client.RunBacktest(ctx, &req)
	if err != nil {
		return nil, fmt.Errorf("error running: %w", err)
	}
	totalPeriods := rsp.NumPeriods

	log.Debug().Any("response", rsp).Msg("run backtest sent")

	ch := make(chan *servicePb.RunExecutionResponse)

	lookup := z.symbolsAt
	var pool *pgxpool.Pool
	if lookup == nil {
		pool, err = pgxpool.New(ctx, environment.GetPostgresURL())
		if err != nil {
			return nil, fmt.Errorf("error connecting to postgres for membership: %w", err)
		}
		lookup = &repository.UniverseMember{Conn: pool}
	}

	runner := func() {
		if pool != nil {
			defer pool.Close()
		}
		var currentPeriod int64 = 0
		for {
			period, err := z.client.GetCurrentPeriod(ctx, &servicePb.GetCurrentPeriodRequest{})
			if err != nil {
				close(ch)
				return
			}
			if !period.IsRunning {
				close(ch)
				return
			}
			currentPeriod++
			excRsp := servicePb.RunExecutionResponse{
				Execution:        execution,
				Portfolio:        period.Portfolio,
				NumTotalPeriods:  totalPeriods,
				NumCurrentPeriod: currentPeriod,
			}

			select {
			case ch <- &excRsp:
			default:
			}
			t := period.Portfolio.Timestamp.AsTime()
			symbols, err := memberSymbolsAt(ctx, lookup, execution.GetBacktest(), t)
			if err != nil {
				log.Error().Err(err).Msg("error listing members at timestamp")
				close(ch)
				return
			}
			orders, err := workers.Process(ctx, t, symbols, period.Portfolio)
			if err != nil {
				log.Error().Err(err).Msg("error processing orders")
				close(ch)
				return
			}

			_, err = z.client.PlaceOrdersAndContinue(ctx,
				&servicePb.PlaceOrdersAndContinueRequest{
					Orders: orders,
				},
			)
			if err != nil {
				log.Error().Err(err).Msg("error placing orders")
				close(ch)

				return
			}
		}
	}
	go runner()

	return ch, nil
}

func memberSymbolsAt(ctx context.Context, lookup MemberSymbols, backtestName string, t time.Time) ([]string, error) {
	if lookup == nil || backtestName == "" {
		return []string{}, nil
	}
	symbols, err := lookup.SymbolsAt(ctx, backtestName, t)
	if err != nil {
		return nil, err
	}
	if symbols == nil {
		return []string{}, nil
	}
	return symbols, nil
}

func (z *ZipelineSession) GetResult(ctx context.Context, executionId string) (*servicePb.GetResultResponse, error) {
	req := servicePb.GetResultRequest{
		Execution: executionId,
		Upload: &servicePb.GetResultRequest_Upload{
			Bucket: string(storage.ResultsBucket),
			Object: executionId,
		},
	}
	rsp, err := z.client.GetResult(ctx, &req)
	if err != nil {
		return nil, fmt.Errorf("error getting result: %w", err)
	}
	return rsp, nil
}
