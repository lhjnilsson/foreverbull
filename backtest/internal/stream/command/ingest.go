package command

import (
	"context"
	"fmt"

	"github.com/lhjnilsson/foreverbull/backtest/engine"
	"github.com/lhjnilsson/foreverbull/backtest/internal/repository"
	"github.com/lhjnilsson/foreverbull/backtest/internal/stream/dependency"
	bs "github.com/lhjnilsson/foreverbull/backtest/stream"
	"github.com/lhjnilsson/foreverbull/internal/postgres"
	"github.com/lhjnilsson/foreverbull/internal/stream"
)

func UpdateIngestStatus(ctx context.Context, message stream.Message) error {
	db := message.MustGet(stream.DBDep).(postgres.Query)

	command := bs.UpdateIngestStatusCommand{}
	err := message.ParsePayload(&command)
	if err != nil {
		return fmt.Errorf("error unmarshalling UpdateIngestStatus payload: %w", err)
	}

	ingestions := repository.Ingestion{Conn: db}
	err = ingestions.UpdateStatus(ctx, command.Name, command.Status, command.Error)
	if err != nil {
		return fmt.Errorf("error updating ingestion status: %w", err)
	}
	return nil
}

func Ingest(ctx context.Context, message stream.Message) error {
	db := message.MustGet(stream.DBDep).(postgres.Query)

	command := bs.IngestCommand{}
	err := message.ParsePayload(&command)
	if err != nil {
		return fmt.Errorf("error unmarshalling MarketdataDownloaded payload: %w", err)
	}

	backtests := repository.Backtest{Conn: db}
	b, err := backtests.Get(ctx, command.Name)
	if err != nil {
		return fmt.Errorf("error getting backtest: %w", err)
	}

	ingest := func(e engine.Engine) error {
		err = e.Ingest(ctx, b)
		if err != nil {
			return fmt.Errorf("error ingesting: %w", err)
		}

		err = e.UploadIngestion(ctx, b.Name)
		if err != nil {
			return fmt.Errorf("error uploading ingestion: %w", err)
		}
		return nil
	}

	engineInstance, err := message.Call(ctx, dependency.GetIngestEngineKey)
	if err != nil {
		return fmt.Errorf("error getting backtest engine: %w", err)
	}
	e := engineInstance.(engine.Engine)
	err = ingest(e)
	if err != nil {
		return fmt.Errorf("error ingesting: %w", err)
	}
	return nil
}
