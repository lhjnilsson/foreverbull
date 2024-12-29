package backtest

import (
	"context"
	"fmt"

	"github.com/lhjnilsson/foreverbull/internal/container"
	"github.com/lhjnilsson/foreverbull/internal/storage"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/engine"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/pb"
	backtest_pb "github.com/lhjnilsson/foreverbull/pkg/backtest/pb"
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

func (z *Zipline) NewSession(ctx context.Context, session *pb.Session) (engine.EngineSession, error) {
	rsp, err := z.client.NewSession(ctx, &backtest_pb.NewSessionRequest{Id: session.Id})
	if err != nil {
		return nil, err
	}
	ip, err := z.container.GetIpAddress()
	if err != nil {
		return nil, err
	}
	connectionStr := fmt.Sprintf("%s:%d", ip, rsp.Port)
	return NewZiplineEngineSession(ctx, connectionStr)
}
