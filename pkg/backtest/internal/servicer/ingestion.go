package servicer

import (
	"context"
	"fmt"
	"strings"

	pb_internal "github.com/lhjnilsson/foreverbull/internal/pb"
	"github.com/lhjnilsson/foreverbull/internal/storage"
	"github.com/lhjnilsson/foreverbull/internal/stream"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/pb"
	bs "github.com/lhjnilsson/foreverbull/pkg/backtest/stream"
)

type IngestionServer struct {
	pb.UnimplementedIngestionServicerServer

	stream  stream.Stream
	storage storage.Storage
}

func NewIngestionServer(stream stream.Stream, storage storage.Storage) *IngestionServer {
	return &IngestionServer{
		stream:  stream,
		storage: storage,
	}
}

func (is *IngestionServer) CreateIngestion(ctx context.Context, req *pb.CreateIngestionRequest) (*pb.CreateIngestionResponse, error) {
	sd := req.Ingestion.GetStartDate()
	ed := req.Ingestion.GetEndDate()
	if sd == nil || ed == nil {
		return nil, fmt.Errorf("start date and end date must be provided")
	}
	symbols := req.Ingestion.GetSymbols()
	if len(symbols) == 0 {
		return nil, fmt.Errorf("at least one symbol must be provided")
	}

	metadata := map[string]string{
		"Start_date": pb_internal.DateToDateString(sd),
		"End_date":   pb_internal.DateToDateString(ed),
		"Symbols":    strings.Join(req.Ingestion.Symbols, ","),
		"Status":     pb.IngestionStatus_CREATED.String(),
	}
	name := fmt.Sprintf("%s-%s", pb_internal.DateToDateString(sd), pb_internal.DateToDateString(ed))
	_, err := is.storage.CreateObject(ctx, storage.IngestionsBucket, name, storage.WithMetadata(metadata))
	if err != nil {
		return nil, fmt.Errorf("error creating ingestion: %w", err)
	}
	o, err := bs.NewIngestOrchestration(name, symbols, pb_internal.DateToDateString(sd), pb_internal.DateToDateString(ed))
	if err != nil {
		return nil, fmt.Errorf("error creating orchestration: %w", err)
	}
	err = is.stream.RunOrchestration(ctx, o)
	if err != nil {
		return nil, fmt.Errorf("error sending orchestration: %w", err)
	}
	return &pb.CreateIngestionResponse{}, nil
}

func (is *IngestionServer) GetCurrentIngestion(ctx context.Context, req *pb.GetCurrentIngestionRequest) (*pb.GetCurrentIngestionResponse, error) {
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
	symbols := strings.Split(ingestion.Metadata["Symbols"], ",")

	var status pb.IngestionStatus
	switch ingestion.Metadata["Status"] {
	case pb.IngestionStatus_CREATED.String():
		status = pb.IngestionStatus_CREATED
	case pb.IngestionStatus_INGESTING.String():
		status = pb.IngestionStatus_INGESTING
	case pb.IngestionStatus_READY.String():
		status = pb.IngestionStatus_READY
	}

	return &pb.GetCurrentIngestionResponse{
		Ingestion: &pb.Ingestion{
			StartDate: pb_internal.DateStringToDate(ingestion.Metadata["Start_date"]),
			EndDate:   pb_internal.DateStringToDate(ingestion.Metadata["End_date"]),
			Symbols:   symbols,
		},
		Status: status,
		Size:   ingestion.Size,
	}, nil
}
