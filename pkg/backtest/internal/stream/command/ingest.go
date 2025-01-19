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
	"github.com/rs/zerolog/log"
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

	setMetadata := func(ctx context.Context, status pb.IngestionStatus) {
		object.Metadata["Status"] = status.String()
		err = object.SetMetadata(ctx, object.Metadata)
		if err != nil {
			log.Err(err).Msg("error updating metadata")
		}
	}

	setMetadata(ctx, pb.IngestionStatus_INGESTING)

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
		"Status":     pb.IngestionStatus_COMPLETED.String(),
	}

	err = object.SetMetadata(ctx, metadata)
	if err != nil {
		return fmt.Errorf("error setting object metadata: %w", err)
	}

	return nil
}

func UpdateIngestionStatus(ctx context.Context, msg stream.Message) error {
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
