package command

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/storage"
	"github.com/lhjnilsson/foreverbull/internal/stream"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/engine"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/repository"
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

	metadata := map[string]string{
		"Start_date": command.Start,
		"End_date":   command.End,
		"Symbols":    strings.Join(command.Symbols, ","),
	}
	name := fmt.Sprintf("%s-%s", command.Start, command.End)

	object, err := store.CreateObject(ctx, storage.IngestionsBucket, name, storage.WithMetadata(metadata))
	if err != nil {
		return fmt.Errorf("error creating ingestion: %w", err)
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

	db := msg.MustGet(stream.DBDep).(*pgxpool.Pool)
	backtests := repository.Backtest{Conn: db}

	err = backtests.UpdateStatus(ctx, command.Name, command.Status, nil)
	if err != nil {
		return fmt.Errorf("error updating backtest status: %w", err)
	}
	return nil
}
