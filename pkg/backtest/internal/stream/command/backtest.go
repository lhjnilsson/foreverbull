package command

import (
	"context"
	"errors"
	"fmt"

	"github.com/lhjnilsson/foreverbull/internal/storage"
	"github.com/lhjnilsson/foreverbull/internal/stream"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/engine"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/stream/dependency"
	ss "github.com/lhjnilsson/foreverbull/pkg/backtest/stream"
	pb_internal "github.com/lhjnilsson/foreverbull/pkg/pb"
	pb "github.com/lhjnilsson/foreverbull/pkg/pb/backtest"
)

func Ingest(ctx context.Context, msg stream.Message) error {
	command := ss.IngestCommand{}

	err := msg.ParsePayload(&command)
	if err != nil {
		return fmt.Errorf("error unmarshalling Ingest payload: %w", err)
	}

	store, isStorage := msg.MustGet(stream.StorageDep).(storage.Storage)
	if !isStorage {
		return errors.New("error casting storage")
	}

	object, err := store.GetObject(ctx, storage.IngestionsBucket, command.Name)
	if err != nil {
		return fmt.Errorf("error getting object from storage: %w", err)
	}

	ze, err := msg.Call(ctx, dependency.GetEngineKey)
	if err != nil {
		return fmt.Errorf("error getting zipline engine: %w", err)
	}

	engine, isEngine := ze.(engine.Engine)
	if !isEngine {
		return errors.New("error casting zipline engine")
	}

	ingestion := pb.Ingestion{
		StartDate: pb_internal.DateStringToDate(command.Start),
		EndDate:   pb_internal.DateStringToDate(command.End),
		Symbols:   command.Symbols,
	}

	err = engine.Ingest(ctx, &ingestion, object)
	if err != nil {
		return fmt.Errorf("error ingesting data: %w", err)
	}
	return nil
}

func UpdateBacktestStatus(ctx context.Context, msg stream.Message) error {
	command := ss.UpdateStatusCommand{}

	err := msg.ParsePayload(&command)
	if err != nil {
		return fmt.Errorf("error unmarshalling Ingest payload: %w", err)
	}

	store, isStorage := msg.MustGet(stream.StorageDep).(storage.Storage)
	if !isStorage {
		return errors.New("error casting storage")
	}

	object, err := store.GetObject(ctx, storage.IngestionsBucket, command.Name)
	if err != nil {
		return fmt.Errorf("error getting object from storage: %w", err)
	}

	object.Metadata["Status"] = command.Status.String()
	err = object.SetMetadata(ctx, object.Metadata)
	if err != nil {
		return fmt.Errorf("error setting object metadata: %w", err)
	}
	return nil
}
