package backtest

import (
	"context"
	"fmt"

	"github.com/lhjnilsson/foreverbull/internal/container"
	"github.com/lhjnilsson/foreverbull/internal/storage"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/engine"
	backtest_pb "github.com/lhjnilsson/foreverbull/pkg/backtest/pb"
	finance_pb "github.com/lhjnilsson/foreverbull/pkg/finance/pb"
	"github.com/lhjnilsson/foreverbull/pkg/service/worker"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

/*
NewZiplineEngine
Returns a Zipline backtest engine.
*/
func NewZiplineEngine(ctx context.Context, container container.Container, ingestionURL *string) (engine.Engine, error) {
	connStr, err := container.GetConnectionString()
	if err != nil {
		return nil, fmt.Errorf("error getting connection string: %w", err)
	}

	conn, err := grpc.NewClient(
		connStr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("error getting grpc client: %w", err)
	}

	client := backtest_pb.NewEngineClient(conn)
	if ingestionURL != nil {
		_, err = client.DownloadIngestion(ctx, &backtest_pb.DownloadIngestionRequest{})
		if err != nil {
			return nil, fmt.Errorf("error downloading ingestion: %w", err)
		}
	}

	z := Zipline{client: client, container: container}

	return &z, nil
}

type Zipline struct {
	client    backtest_pb.EngineClient
	container container.Container
}

func (z *Zipline) Ingest(ctx context.Context, ingestion *backtest_pb.Ingestion, object *storage.Object) error {
	bucket := string(object.Bucket)
	req := backtest_pb.IngestRequest{
		Ingestion: ingestion,
		Bucket:    &bucket,
		Object:    &object.Name,
	}

	_, err := z.client.Ingest(ctx, &req)
	if err != nil {
		return fmt.Errorf("error ingesting: %w", err)
	}

	return nil
}

func (z *Zipline) DownloadIngestion(ctx context.Context, object *storage.Object) error {
	bucket := string(object.Bucket)
	req := backtest_pb.DownloadIngestionRequest{
		Bucket: bucket,
		Object: object.Name,
	}

	_, err := z.client.DownloadIngestion(ctx, &req)
	if err != nil {
		return fmt.Errorf("error downloading ingestion: %w", err)
	}

	return nil
}

func (z *Zipline) RunBacktest(ctx context.Context, backtest *backtest_pb.Backtest,
	workers worker.Pool,
) (chan *finance_pb.Portfolio, error) {
	req := backtest_pb.RunRequest{
		Backtest: backtest,
	}

	rsp, err := z.client.RunBacktest(ctx, &req)
	if err != nil {
		return nil, fmt.Errorf("error running: %w", err)
	}

	log.Debug().Any("response", rsp).Msg("run backtest sent")

	portfolioCh := make(chan *finance_pb.Portfolio)

	runner := func() {
		for {
			period, err := z.client.GetCurrentPeriod(ctx, &backtest_pb.GetCurrentPeriodRequest{})
			if err != nil {
				close(portfolioCh)
				return
			}

			if !period.IsRunning {
				close(portfolioCh)
				return
			}
			select {
			case portfolioCh <- period.Portfolio:
			default:
			}

			orders, err := workers.Process(ctx, period.Portfolio.Timestamp.AsTime(), backtest.Symbols, period.Portfolio)
			if err != nil {
				log.Error().Err(err).Msg("error processing orders")
				close(portfolioCh)

				return
			}

			_, err = z.client.PlaceOrdersAndContinue(ctx,
				&backtest_pb.PlaceOrdersAndContinueRequest{
					Orders: orders,
				},
			)
			if err != nil {
				log.Error().Err(err).Msg("error placing orders")
				close(portfolioCh)

				return
			}
		}
	}
	go runner()

	return portfolioCh, nil
}

func (z *Zipline) GetResult(ctx context.Context) (*backtest_pb.GetResultResponse, error) {
	req := backtest_pb.GetResultRequest{}

	rsp, err := z.client.GetResult(ctx, &req)
	if err != nil {
		return nil, fmt.Errorf("error getting result: %w", err)
	}

	return rsp, nil
}

func (z *Zipline) Stop(ctx context.Context) error {
	req := backtest_pb.StopRequest{}

	_, err := z.client.Stop(ctx, &req)
	if err != nil {
		return fmt.Errorf("error stopping: %w", err)
	}

	return nil
}
