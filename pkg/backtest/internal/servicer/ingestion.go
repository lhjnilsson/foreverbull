package servicer

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/storage"
	"github.com/lhjnilsson/foreverbull/internal/stream"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/repository"
	bs "github.com/lhjnilsson/foreverbull/pkg/backtest/stream"
	pb_internal "github.com/lhjnilsson/foreverbull/pkg/pb"
	pb "github.com/lhjnilsson/foreverbull/pkg/pb/backtest"
)

type IngestionServer struct {
	pb.UnimplementedIngestionServicerServer

	pgx     *pgxpool.Pool
	stream  stream.Stream
	storage storage.Storage
}

func NewIngestionServer(stream stream.Stream, storage storage.Storage, pgx *pgxpool.Pool) *IngestionServer {
	return &IngestionServer{
		pgx:     pgx,
		stream:  stream,
		storage: storage,
	}
}

func statusToPbStatus(status string) pb.IngestionStatus {
	var pbStatus pb.IngestionStatus
	switch status {
	case pb.IngestionStatus_CREATED.String():
		pbStatus = pb.IngestionStatus_CREATED
	case pb.IngestionStatus_DOWNLOADING.String():
		pbStatus = pb.IngestionStatus_DOWNLOADING
	case pb.IngestionStatus_INGESTING.String():
		pbStatus = pb.IngestionStatus_INGESTING
	case pb.IngestionStatus_COMPLETED.String():
		pbStatus = pb.IngestionStatus_COMPLETED
	case pb.IngestionStatus_ERROR.String():
		pbStatus = pb.IngestionStatus_ERROR
	}
	return pbStatus
}

func (is *IngestionServer) UpdateIngestion(req *pb.UpdateIngestionRequest,
	stream pb.IngestionServicer_UpdateIngestionServer,
) error {
	backtests := repository.Backtest{Conn: is.pgx}

	start, end, symbols, err := backtests.GetUniverse(stream.Context())
	if err != nil {
		return fmt.Errorf("error getting universe: %w", err)
	}

	metadata := map[string]string{
		"Start_date": pb_internal.DateToDateString(start),
		"End_date":   pb_internal.DateToDateString(end),
		"Symbols":    strings.Join(symbols, ","),
		"Status":     pb.IngestionStatus_CREATED.String(),
	}
	name := fmt.Sprintf("%s-%s", pb_internal.DateToDateString(start), pb_internal.DateToDateString(end))

	_, err = is.storage.CreateObject(stream.Context(), storage.IngestionsBucket, name, storage.WithMetadata(metadata))
	if err != nil {
		return fmt.Errorf("error creating ingestion: %w", err)
	}

	orchestration, err := bs.NewIngestOrchestration(name, symbols,
		pb_internal.DateToDateString(start), pb_internal.DateToDateString(end))
	if err != nil {
		return fmt.Errorf("error creating orchestration: %w", err)
	}

	err = is.stream.RunOrchestration(stream.Context(), orchestration)
	if err != nil {
		return fmt.Errorf("error sending orchestration: %w", err)
	}

	err = stream.Send(&pb.UpdateIngestionResponse{
		Ingestion: &pb.Ingestion{},
		Status:    pb.IngestionStatus_CREATED,
	})
	if err != nil {
		return fmt.Errorf("Error sending: %w", err)
	}

	var latestStatus = pb.IngestionStatus_CREATED

	for {
		ingestion, err := is.storage.GetObject(stream.Context(), storage.IngestionsBucket, name)
		if err != nil {
			return fmt.Errorf("Error getting ingestion: %w", err)
		}
		status := statusToPbStatus(ingestion.Metadata["Status"])
		if status == latestStatus {
			time.Sleep(time.Second / 2)
			continue
		}
		latestStatus = status
		err = stream.Send(&pb.UpdateIngestionResponse{
			Ingestion: &pb.Ingestion{
				StartDate: pb_internal.DateStringToDate(ingestion.Metadata["Start_date"]),
				EndDate:   pb_internal.DateStringToDate(ingestion.Metadata["End_date"]),
				Symbols:   strings.Split(ingestion.Metadata["Symbols"], ","),
			},
			Status:       status,
			ErrorMessage: ingestion.Metadata["Symbols"],
		})
		if err != nil {
			return fmt.Errorf("Error sending: %w", err)
		}
		if status == pb.IngestionStatus_COMPLETED || status == pb.IngestionStatus_ERROR {
			break
		}
	}
	return nil
}

func (is *IngestionServer) GetCurrentIngestion(ctx context.Context,
	_ *pb.GetCurrentIngestionRequest,
) (*pb.GetCurrentIngestionResponse, error) {
	ingestions, err := is.storage.ListObjects(ctx, storage.IngestionsBucket)
	if err != nil {
		return nil, fmt.Errorf("error listing ingestions: %w", err)
	}

	if len(*ingestions) == 0 {
		return &pb.GetCurrentIngestionResponse{
			Ingestion: nil,
		}, nil
	}

	var ingestion *storage.Object
	for _, i := range *ingestions {
		if ingestion == nil {
			ingestion = &i
		} else if i.LastModified.After(ingestion.LastModified) {
			ingestion = &i
		}
	}

	if len(ingestion.Metadata) == 0 {
		err = ingestion.Refresh() // Objects from list does not include metadata, refresh to obtain
		if err != nil {
			return nil, fmt.Errorf("error refreshing ingestion: %w", err)
		}
	}

	return &pb.GetCurrentIngestionResponse{
		Ingestion: &pb.Ingestion{
			StartDate: pb_internal.DateStringToDate(ingestion.Metadata["Start_date"]),
			EndDate:   pb_internal.DateStringToDate(ingestion.Metadata["End_date"]),
			Symbols:   strings.Split(ingestion.Metadata["Symbols"], ","),
		},
		Status: statusToPbStatus(ingestion.Metadata["Status"]),
		Size:   ingestion.Size,
	}, nil
}
