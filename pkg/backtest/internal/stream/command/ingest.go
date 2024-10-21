package command

import (
	"context"
	"errors"
	"fmt"
	"strings"

	pb_internal "github.com/lhjnilsson/foreverbull/internal/pb"
	"github.com/lhjnilsson/foreverbull/internal/storage"
	"github.com/lhjnilsson/foreverbull/internal/stream"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/engine"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/stream/dependency"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/pb"
	ss "github.com/lhjnilsson/foreverbull/pkg/backtest/stream"
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

	object.Metadata["Status"] = pb.IngestionStatus_INGESTING.String()

	err = object.SetMetadata(ctx, object.Metadata)
	if err != nil {
		return fmt.Errorf("error setting object metadata: %w", err)
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

	err = object.Refresh()
	if err != nil {
		return fmt.Errorf("error refreshing object: %w", err)
	}

	metadata := map[string]string{
		"Start_date": command.Start,
		"End_date":   command.End,
		"Symbols":    strings.Join(command.Symbols, ","),
		"Status":     pb.IngestionStatus_READY.String(),
	}

	err = object.SetMetadata(ctx, metadata)
	if err != nil {
		return fmt.Errorf("error setting object metadata: %w", err)
	}

	return nil
}
