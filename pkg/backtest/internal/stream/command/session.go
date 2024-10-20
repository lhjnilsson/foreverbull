package command

import (
	"context"
	"errors"
	"fmt"
	"net"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/internal/storage"
	"github.com/lhjnilsson/foreverbull/internal/stream"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/engine"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/backtest"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/repository"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/stream/dependency"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/pb"
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
		sessions.UpdateStatus(ctx, command.SessionID, pb.Session_Status_FAILED, err)

		return fmt.Errorf("error getting zipline engine: %w", err)
	}

	engine := depEngine.(engine.Engine)

	var ingestions *[]storage.Object

	ingestions, err = s.ListObjects(ctx, storage.IngestionsBucket)
	if err != nil {
		log.Err(err).Msg("error listing ingestions")
		sessions.UpdateStatus(ctx, command.SessionID, pb.Session_Status_FAILED, err)

		return fmt.Errorf("error listing ingestions: %w", err)
	}

	if len(*ingestions) == 0 {
		err = errors.New("no ingestions found")
		sessions.UpdateStatus(ctx, command.SessionID, pb.Session_Status_FAILED, err)
		log.Err(err).Msg("no ingestions found")

		return fmt.Errorf("no ingestions found")
	}

	var ingestion *storage.Object

	for _, stored := range *ingestions {
		stored.Refresh()

		if ingestion == nil && stored.Metadata["Status"] == pb.IngestionStatus_READY.String() {
			ingestion = &stored
		} else if ingestion != nil && stored.LastModified.After(ingestion.LastModified) && stored.Metadata["Status"] == pb.IngestionStatus_READY.String() {
			ingestion = &stored
		}
	}

	if ingestion == nil {
		err = errors.New("no completed ingestions found, create one before running a session")
		sessions.UpdateStatus(ctx, command.SessionID, pb.Session_Status_FAILED, err)

		return err
	}

	err = engine.DownloadIngestion(ctx, ingestion)
	if err != nil {
		log.Err(err).Msg("error downloading ingestion")
		sessions.UpdateStatus(ctx, command.SessionID, pb.Session_Status_FAILED, err)

		return fmt.Errorf("error downloading ingestion: %w", err)
	}

	server, activity, err := backtest.NewGRPCSessionServer(session, db, engine)
	if err != nil {
		log.Err(err).Msg("error creating grpc session server")
		sessions.UpdateStatus(ctx, command.SessionID, pb.Session_Status_FAILED, err)

		return fmt.Errorf("error creating grpc session server: %w", err)
	}

	var listener net.Listener

	var port int

	portRangeEnd := environment.GetBacktestPortRangeEnd()

	for port = environment.GetBacktestPortRangeStart(); port < portRangeEnd; port++ {
		listener, err = net.Listen("tcp", fmt.Sprintf(":%d", port))
		if err == nil {
			break
		}

		opErr := &net.OpError{}
		if errors.As(err, &opErr) && errors.Unwrap(opErr.Unwrap()) == syscall.EADDRINUSE {
			continue
		}

		return fmt.Errorf("error creating listener: %w", err)
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
		sessions.UpdateStatus(ctx, command.SessionID, pb.Session_Status_RUNNING, nil)

		defer sessions.UpdateStatus(ctx, command.SessionID, pb.Session_Status_COMPLETED, nil)

		for {
			select {
			case _, active := <-activity:
				if !active {
					time.Sleep(time.Second / 4) // make sure reply is sent
					return
				} else {
				}
			case <-time.After(SessionTimeout):
				return
			}
		}
	}()
	sessions.UpdatePort(ctx, command.SessionID, port)

	return nil
}
