package command

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/auth"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/internal/socket"
	"github.com/lhjnilsson/foreverbull/internal/storage"
	"github.com/lhjnilsson/foreverbull/internal/stream"
	"github.com/lhjnilsson/foreverbull/internal/usercopy"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/engine"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/api"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/repository"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/stream/dependency"
	ss "github.com/lhjnilsson/foreverbull/pkg/backtest/stream"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
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

	var secret string
	if environment.GetAuthEnabled() {
		looked, ok := auth.LookupBacktestSecret(command.SessionID)
		if !ok {
			return fmt.Errorf("%s is missing", auth.SessionMetadata())
		}
		secret = looked
	}

	ingestion, err := sessionIngestion(ctx, sessions, s, session.GetId(), session.GetBacktest())
	if err != nil {
		return err
	}
	if err = engine.DownloadIngestion(ctx, ingestion); err != nil {
		log.Err(err).Msg("error downloading ingestion")
		return fmt.Errorf("error downloading ingestion: %w", err)
	}

	var server *grpc.Server
	var activity <-chan bool
	if environment.GetAuthEnabled() {
		server, activity, err = api.NewSecuredGRPCSessionServer(session, db, engine, secret)
	} else {
		server, activity, err = api.NewGRPCSessionServer(session, db, engine)
	}
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
			if environment.GetAuthEnabled() {
				auth.ForgetBacktestSecret(command.SessionID)
			}
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

	return publishSessionPort(ctx, &sessions, server, command.SessionID, port)
}

type sessionPortStore interface {
	UpdatePort(ctx context.Context, sessionID string, port int) error
}

// publishSessionPort stores the port the session server listens on. No
// client can reach a server whose port isn't stored, so a failed write
// stops the server and fails the command.
func publishSessionPort(ctx context.Context, sessions sessionPortStore, server *grpc.Server, sessionID string, port int) error {
	if err := sessions.UpdatePort(ctx, sessionID, port); err != nil {
		server.Stop()
		return stream.Unreachable(fmt.Errorf("error storing session port: %w", err))
	}
	return nil
}

// ingestionKey is the object key for a backtest's ingestion. Both auth modes
// use the backtest id. A name is not a key: a re-create would otherwise read
// an object left under the previous name. An empty id is an error.
func ingestionKey(id string) (string, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return "", errors.New("backtest id is empty")
	}
	return id, nil
}

func sessionIngestion(ctx context.Context, sessions repository.Session, store storage.Storage, sessionID, backtestName string) (*storage.Object, error) {
	var id string
	var err error
	if environment.GetAuthEnabled() {
		id, err = (&sessions).BacktestID(ctx, sessionID)
	} else {
		id, err = (&repository.Backtest{Conn: sessions.Conn}).IDByName(ctx, backtestName)
	}
	if err != nil {
		return nil, fmt.Errorf("error getting session backtest: %w", err)
	}
	key, err := ingestionKey(id)
	if err != nil {
		return nil, err
	}
	return ingestionByKey(ctx, store, backtestName, key)
}

// ingestionByKey loads one backtest's ingestion. A missing object is that
// backtest's failure. Another backtest's object is not a substitute.
func ingestionByKey(ctx context.Context, store storage.Storage, backtestName, key string) (*storage.Object, error) {
	ingestion, err := store.GetObject(ctx, storage.IngestionsBucket, key)
	if err != nil {
		if storage.MissingObject(err) {
			log.Err(err).Str("backtest", backtestName).Msg("no ingestion for backtest")
			return nil, errors.New(usercopy.BacktestIngestionMissing(backtestName))
		}
		return nil, fmt.Errorf("error getting ingestion for backtest %s: %w", backtestName, err)
	}
	if err := ingestion.Refresh(); err != nil {
		return nil, fmt.Errorf("error refreshing ingestion for backtest %s: %w", backtestName, err)
	}
	return ingestion, nil
}
