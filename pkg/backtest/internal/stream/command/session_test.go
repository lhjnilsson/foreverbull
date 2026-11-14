package command_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/auth"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/internal/storage"
	"github.com/lhjnilsson/foreverbull/internal/stream"
	"github.com/lhjnilsson/foreverbull/internal/test_helper"
	"github.com/lhjnilsson/foreverbull/internal/usercopy"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/engine"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/repository"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/stream/command"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/stream/dependency"
	ss "github.com/lhjnilsson/foreverbull/pkg/backtest/stream"
	"github.com/lhjnilsson/foreverbull/pkg/pb/backtest"
	servicePb "github.com/lhjnilsson/foreverbull/pkg/pb/backtest/service"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type CommandSessionTest struct {
	suite.Suite

	db       *pgxpool.Pool
	backtest *backtest.Backtest
	session  *backtest.Session
	storage  *storage.MockStorage
}

func TestCommandSession(t *testing.T) {
	suite.Run(t, new(CommandSessionTest))
}

func (test *CommandSessionTest) SetupSuite() {
	test_helper.SetupEnvironment(test.T(), &test_helper.Containers{
		Postgres: true,
		Minio:    true,
	})
}

func (test *CommandSessionTest) SetupSubTest() {
	var err error
	test.db, err = pgxpool.New(context.Background(), environment.GetPostgresURL())
	test.Require().NoError(err)

	test.storage = new(storage.MockStorage)

	err = repository.Recreate(context.TODO(), test.db)
	test.Require().NoError(err)
	backtests := repository.Backtest{Conn: test.db}
	backtest := backtest.Backtest{
		Name: "test-backtest",
	}
	test.backtest, err = backtests.Create(context.TODO(), &backtest)
	test.Require().NoError(err)
	sessions := repository.Session{Conn: test.db}
	test.session, err = sessions.Create(context.TODO(), "test-backtest")
	test.Require().NoError(err)
}

func (test *CommandSessionTest) TearDownSubTest() {
}

func (test *CommandSessionTest) TestSessionRun() {
	test.Run("Fail to unmarshal", func() {
		message := new(stream.MockMessage)
		parse := message.On("ParsePayload", &ss.SessionRunCommand{}).Return(errors.New("not working"))
		err := command.SessionRun(context.TODO(), message)
		test.Require().Error(err)
		test.Require().Len(parse.Parent.Calls, 1)
	})
	test.Run("Session not stored", func() {
		message := new(stream.MockMessage)
		message.On("MustGet", stream.DBDep).Return(test.db)
		message.On("MustGet", stream.StorageDep).Return(test.storage)
		message.On("ParsePayload", &ss.SessionRunCommand{}).Return(nil).Run(func(args mock.Arguments) {
			payload := args.Get(0).(*ss.SessionRunCommand)
			payload.Backtest = test.backtest.Name
			payload.SessionID = "not stored"
		})

		err := command.SessionRun(context.TODO(), message)
		test.Require().Error(err)
		message.AssertCalled(test.T(), "ParsePayload", mock.Anything)
		message.AssertCalled(test.T(), "MustGet", stream.DBDep)
	})
	test.Run("Fail to get engine", func() {
		message := new(stream.MockMessage)
		message.On("MustGet", stream.DBDep).Return(test.db)
		message.On("MustGet", stream.StorageDep).Return(test.storage)
		message.On("ParsePayload", &ss.SessionRunCommand{}).Return(nil).Run(func(args mock.Arguments) {
			payload := args.Get(0).(*ss.SessionRunCommand)
			payload.Backtest = test.backtest.Name
			payload.SessionID = test.session.Id
		})
		message.On("Call", mock.Anything, dependency.GetEngineKey).Return(nil, errors.New("not working"))
		err := command.SessionRun(context.TODO(), message)
		test.Require().Error(err)
		message.AssertCalled(test.T(), "ParsePayload", mock.Anything)
		message.AssertCalled(test.T(), "MustGet", stream.DBDep)
		message.AssertCalled(test.T(), "Call", mock.Anything, dependency.GetEngineKey)
	})
	test.Run("no ingestion for this backtest", func() {
		test.T().Setenv(environment.AuthEnabled, "false")
		own := "missing-" + uuid.NewString()
		other := "newer-" + uuid.NewString()
		session := test.backtestSession(own)
		_, err := (&repository.Backtest{Conn: test.db}).Create(context.Background(), &backtest.Backtest{Name: other})
		test.Require().NoError(err)
		store := test.minio()
		_, err = store.CreateObject(context.Background(), storage.IngestionsBucket, other)
		test.Require().NoError(err)

		eng := new(engine.MockEngine)
		err = command.SessionRun(context.Background(), test.sessionRunMessage(store, session.GetId(), own, eng))
		test.Require().EqualError(err, usercopy.BacktestIngestionMissing(own))
		eng.AssertNotCalled(test.T(), "DownloadIngestion", mock.Anything, mock.Anything)

		kept, err := store.GetObject(context.Background(), storage.IngestionsBucket, other)
		test.Require().NoError(err)
		test.Equal(other, kept.Name)
	})
	test.Run("uses this backtest's ingestion, not the newest", func() {
		test.T().Setenv(environment.AuthEnabled, "false")
		own := "own-" + uuid.NewString()
		other := "newer-" + uuid.NewString()
		session := test.backtestSession(own)
		_, err := (&repository.Backtest{Conn: test.db}).Create(context.Background(), &backtest.Backtest{Name: other})
		test.Require().NoError(err)
		store := test.minio()
		_, err = store.CreateObject(context.Background(), storage.IngestionsBucket, own)
		test.Require().NoError(err)
		time.Sleep(time.Second)
		_, err = store.CreateObject(context.Background(), storage.IngestionsBucket, other)
		test.Require().NoError(err)
		ownObject, err := store.GetObject(context.Background(), storage.IngestionsBucket, own)
		test.Require().NoError(err)
		otherObject, err := store.GetObject(context.Background(), storage.IngestionsBucket, other)
		test.Require().NoError(err)
		test.Require().True(otherObject.LastModified.After(ownObject.LastModified), "the other ingestion is newer")

		var downloaded *storage.Object
		eng := new(engine.MockEngine)
		engineSession := new(engine.MockEngineSession)
		eng.On("NewSession", mock.Anything, mock.Anything).Return(engineSession, nil)
		eng.On("DownloadIngestion", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
			downloaded = args.Get(1).(*storage.Object)
		}).Return(nil)
		eng.On("Stop", mock.Anything).Return(nil)

		err = command.SessionRun(context.Background(), test.sessionRunMessage(store, session.GetId(), own, eng))
		test.Require().NoError(err)
		test.Require().NotNil(downloaded)
		test.Require().Equal(own, downloaded.Name)

		test.stopSession(session.GetId())
	})
	test.Run("auth on loads the backtest id", func() {
		own := "auth-" + uuid.NewString()
		session := test.backtestSession(own)
		backtestID := "id-" + uuid.NewString()
		_, err := test.db.Exec(context.Background(), `ALTER TABLE backtest ADD COLUMN id text`)
		test.Require().NoError(err)
		_, err = test.db.Exec(context.Background(), `UPDATE backtest SET id = $1 WHERE name = $2`, backtestID, own)
		test.Require().NoError(err)
		_, err = test.db.Exec(context.Background(), `ALTER TABLE session ADD COLUMN backtest_id text`)
		test.Require().NoError(err)
		_, err = test.db.Exec(context.Background(), `UPDATE session SET backtest_id = $1 WHERE id = $2`, backtestID, session.GetId())
		test.Require().NoError(err)
		_, err = auth.MintBacktestSecret(session.GetId())
		test.Require().NoError(err)
		test.T().Cleanup(func() { auth.ForgetBacktestSecret(session.GetId()) })

		store := test.minio()
		_, err = store.CreateObject(context.Background(), storage.IngestionsBucket, backtestID)
		test.Require().NoError(err)
		time.Sleep(time.Second)
		_, err = store.CreateObject(context.Background(), storage.IngestionsBucket, own)
		test.Require().NoError(err)

		var downloaded *storage.Object
		eng := new(engine.MockEngine)
		eng.On("DownloadIngestion", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
			downloaded = args.Get(1).(*storage.Object)
		}).Return(errors.New("stop after lookup"))
		test.T().Setenv(environment.AuthEnabled, "true")
		err = command.SessionRun(context.Background(), test.sessionRunMessage(store, session.GetId(), own, eng))
		test.Require().ErrorContains(err, "error downloading ingestion")
		test.Require().NotNil(downloaded)
		test.Require().Equal(backtestID, downloaded.Name)
	})
	test.Run("refresh failure is returned", func() {
		test.T().Setenv(environment.AuthEnabled, "false")
		own := "refresh-" + uuid.NewString()
		session := test.backtestSession(own)
		store := test.minio()
		_, err := store.CreateObject(context.Background(), storage.IngestionsBucket, own)
		test.Require().NoError(err)

		eng := new(engine.MockEngine)
		err = command.SessionRun(context.Background(), test.sessionRunMessage(dropObjectAfterGet{store}, session.GetId(), own, eng))
		test.Require().Error(err)
		test.Require().ErrorContains(err, "error refreshing ingestion for backtest "+own)
		test.Require().ErrorContains(err, "error refreshing object")
		test.Require().NotContains(err.Error(), "prices aren't loaded")
		eng.AssertNotCalled(test.T(), "DownloadIngestion", mock.Anything, mock.Anything)
	})
}

func (test *CommandSessionTest) minio() storage.Storage {
	store, err := storage.NewMinioStorage(context.Background())
	test.Require().NoError(err)
	return store
}

func (test *CommandSessionTest) backtestSession(name string) *backtest.Session {
	_, err := (&repository.Backtest{Conn: test.db}).Create(context.Background(), &backtest.Backtest{Name: name})
	test.Require().NoError(err)
	session, err := (&repository.Session{Conn: test.db}).Create(context.Background(), name)
	test.Require().NoError(err)
	return session
}

func (test *CommandSessionTest) sessionRunMessage(store storage.Storage, sessionID, name string, eng *engine.MockEngine) *stream.MockMessage {
	message := new(stream.MockMessage)
	message.On("MustGet", stream.DBDep).Return(test.db)
	message.On("MustGet", stream.StorageDep).Return(store)
	message.On("ParsePayload", &ss.SessionRunCommand{}).Return(nil).Run(func(args mock.Arguments) {
		payload := args.Get(0).(*ss.SessionRunCommand)
		payload.Backtest = name
		payload.SessionID = sessionID
	})
	message.On("Call", mock.Anything, dependency.GetEngineKey).Return(eng, nil)
	return message
}

func (test *CommandSessionTest) stopSession(sessionID string) {
	time.Sleep(time.Second / 2)
	sessions := repository.Session{Conn: test.db}
	session, err := sessions.Get(context.Background(), sessionID)
	test.Require().NoError(err)
	test.Require().NotNil(session.Port)

	conn, err := grpc.NewClient(
		fmt.Sprintf("localhost:%d", *session.Port),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	test.Require().NoError(err)
	client := servicePb.NewSessionServicerClient(conn)
	rsp, err := client.StopServer(context.Background(), &servicePb.StopServerRequest{})
	test.Require().NoError(err)
	test.NotNil(rsp)
	time.Sleep(time.Second)
}

// dropObjectAfterGet deletes the object after it is read, so Refresh's stat fails.
type dropObjectAfterGet struct {
	storage.Storage
}

func (d dropObjectAfterGet) GetObject(ctx context.Context, bucket storage.Bucket, name string) (*storage.Object, error) {
	obj, err := d.Storage.GetObject(ctx, bucket, name)
	if err != nil {
		return nil, err
	}
	if err := d.DeleteObject(ctx, bucket, name); err != nil {
		return nil, err
	}
	return obj, nil
}
