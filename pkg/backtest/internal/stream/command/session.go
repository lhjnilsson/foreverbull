package command

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/socket"
	"github.com/lhjnilsson/foreverbull/internal/storage"
	"github.com/lhjnilsson/foreverbull/internal/stream"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/engine"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/api"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/repository"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/stream/dependency"
	ss "github.com/lhjnilsson/foreverbull/pkg/backtest/stream"
	"github.com/rs/zerolog/log"
)

const (
	SessionTimeout = 30 * time.Minute
)

func SessionRun(ctx context.Context, msg stream.Message) error {
	command := ss.SessionRunCommand{}

	err := msg.ParsePayload(&command)
	if err != nil {
		return fmt.Errorf("error unmarshalling SessionRun payload: %w", err)
	}

	db := msg.MustGet(stream.DBDep).(*pgxpool.Pool)
	s := msg.MustGet(stream.StorageDep).(storage.Storage)
	sessions := repository.Session{Conn: db}

	session, err := sessions.Get(ctx, command.SessionID)
	if err != nil {
		log.Err(err).Msg("error getting session")
		return fmt.Errorf("error getting session: %w", err)
	}

	depEngine, err := msg.Call(ctx, dependency.GetEngineKey)
	if err != nil {
		log.Err(err).Msg("error getting zipline engine")
		return fmt.Errorf("error getting zipline engine: %w", err)
	}

	engine := depEngine.(engine.Engine)

	var ingestions *[]storage.Object

	ingestions, err = s.ListObjects(ctx, storage.IngestionsBucket)
	if err != nil {
		log.Err(err).Msg("error listing ingestions")
		return fmt.Errorf("error listing ingestions: %w", err)
	}

	if len(*ingestions) == 0 {
		return errors.New("no ingestions found")
	}

	var ingestion *storage.Object

	for _, stored := range *ingestions {
		stored.Refresh() //nolint: errcheck
		if ingestion == nil || stored.LastModified.After(ingestion.LastModified) {
			ingestion = &stored
		}

	}

	if ingestion == nil {
		err = errors.New("no completed ingestions found, create one before running a session")
		return err
	}

	err = engine.DownloadIngestion(ctx, ingestion)
	if err != nil {
		log.Err(err).Msg("error downloading ingestion")
		return fmt.Errorf("error downloading ingestion: %w", err)
	}

	server, activity, err := api.NewGRPCSessionServer(session, db, engine)
	if err != nil {
		log.Err(err).Msg("error creating grpc session server")
		return fmt.Errorf("error creating grpc session server: %w", err)
	}

	listener, port, err := socket.ListenTCP("")
	if err != nil {
		return fmt.Errorf("error creating session listener: %w", err)
	}

	go func() {
		err := server.Serve(listener)
		if err != nil {
			log.Error().Err(err).Msg("error serving session server")
		}
	}()
	go func() {
		defer func() {
			log.Info().Msg("closing session server")
			server.Stop()
		}()

		for {
			select {
			case _, active := <-activity:
				if !active {
					time.Sleep(time.Second / 4) // make sure reply is sent
					return
				}
			case <-time.After(SessionTimeout):
				return
			}
		}
	}()

	if inErr := sessions.UpdatePort(ctx, command.SessionID, port); inErr != nil {
		log.Err(inErr).Msg("error updating session status")
	}

	return nil
}
