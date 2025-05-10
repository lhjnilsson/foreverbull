package backtest

import (
	"context"
	"fmt"

	"github.com/lhjnilsson/foreverbull/internal/storage"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/engine"
	backtest_pb "github.com/lhjnilsson/foreverbull/pkg/pb/backtest"
	pb "github.com/lhjnilsson/foreverbull/pkg/pb/backtest"
	"github.com/lhjnilsson/foreverbull/pkg/service/worker"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func NewZiplineEngineSession(ctx context.Context, connStr string) (engine.EngineSession, error) {
	conn, err := grpc.NewClient(
		connStr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("error getting grpc client: %w", err)
	}

	client := pb.NewEngineSessionClient(conn)

	return &ZipelineSession{client: client}, nil
}

type ZipelineSession struct {
	client pb.EngineSessionClient
}

func (z *ZipelineSession) RunBacktest(ctx context.Context, backtest *pb.Backtest, workers worker.Pool) (chan *backtest_pb.RunExecutionResponse, error) {
	req := backtest_pb.RunBacktestRequest{
		Backtest: backtest,
	}

	rsp, err := z.client.RunBacktest(ctx, &req)
	if err != nil {
		return nil, fmt.Errorf("error running: %w", err)
	}
	totalPeriods := rsp.NumPeriods

	log.Debug().Any("response", rsp).Msg("run backtest sent")

	ch := make(chan *backtest_pb.RunExecutionResponse)

	runner := func() {
		var currentPeriod int64 = 0
		for {
			period, err := z.client.GetCurrentPeriod(ctx, &backtest_pb.GetCurrentPeriodRequest{})
			if err != nil {
				close(ch)
				return
			}
			if !period.IsRunning {
				close(ch)
				return
			}
			currentPeriod++
			excRsp := backtest_pb.RunExecutionResponse{
				Backtest:         backtest,
				Portfolio:        period.Portfolio,
				NumTotalPeriods:  totalPeriods,
				NumCurrentPeriod: currentPeriod,
			}

			select {
			case ch <- &excRsp:
			default:
			}
			orders, err := workers.Process(ctx, period.Portfolio.Timestamp.AsTime(), backtest.Symbols, period.Portfolio)
			if err != nil {
				log.Error().Err(err).Msg("error processing orders")
				close(ch)
				return
			}

			_, err = z.client.PlaceOrdersAndContinue(ctx,
				&backtest_pb.PlaceOrdersAndContinueRequest{
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

func (z *ZipelineSession) GetResult(ctx context.Context, executionId string) (*backtest_pb.GetResultResponse, error) {
	req := backtest_pb.GetResultRequest{
		Execution: executionId,
		Upload: &backtest_pb.GetResultRequest_Upload{
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
