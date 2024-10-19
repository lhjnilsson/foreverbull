package command_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/lhjnilsson/foreverbull/pkg/backtest/engine"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	common_pb "github.com/lhjnilsson/foreverbull/internal/pb"
	"github.com/lhjnilsson/foreverbull/internal/storage"
	"github.com/lhjnilsson/foreverbull/internal/stream"
	"github.com/lhjnilsson/foreverbull/internal/test_helper"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/repository"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/stream/command"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/stream/dependency"
	ss "github.com/lhjnilsson/foreverbull/pkg/backtest/stream"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type CommandSessionTest struct {
	suite.Suite

	db       *pgxpool.Pool
	backtest *pb.Backtest
	session  *pb.Session
	storage  *storage.MockStorage
}

func TestCommandSession(t *testing.T) {
	suite.Run(t, new(CommandSessionTest))
}

func (test *CommandSessionTest) SetupSuite() {
	test_helper.SetupEnvironment(test.T(), &test_helper.Containers{
		Postgres: true,
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
	test.backtest, err = backtests.Create(context.TODO(), "test-backtest", &common_pb.Date{Year: 2024, Month: 01, Day: 01}, &common_pb.Date{Year: 2024, Month: 01, Day: 01}, []string{"AAPL"}, nil)
	test.Require().NoError(err)
	sessions := repository.Session{Conn: test.db}
	test.session, err = sessions.Create(context.TODO(), "test-backtest")
}

func (test *CommandSessionTest) TearDownSubTest() {
}

func (test *CommandSessionTest) TestSessionRun() {
	test.Run("Fail to unmarshal", func() {
		m := new(stream.MockMessage)
		parse := m.On("ParsePayload", &ss.SessionRunCommand{}).Return(errors.New("not working"))
		err := command.SessionRun(context.TODO(), m)
		test.Require().Error(err)
		test.Require().Len(parse.Parent.Calls, 1)
	})
	test.Run("Session not stored", func() {
		m := new(stream.MockMessage)
		m.On("MustGet", stream.DBDep).Return(test.db)
		m.On("MustGet", stream.StorageDep).Return(test.storage)
		m.On("ParsePayload", &ss.SessionRunCommand{}).Return(nil).Run(func(args mock.Arguments) {
			payload := args.Get(0).(*ss.SessionRunCommand)
			payload.Backtest = test.backtest.Name
			payload.SessionID = "not stored"
		})

		err := command.SessionRun(context.TODO(), m)
		test.Require().Error(err)
		m.AssertCalled(test.T(), "ParsePayload", mock.Anything)
		m.AssertCalled(test.T(), "MustGet", stream.DBDep)
	})
	test.Run("Fail to get engine", func() {
		m := new(stream.MockMessage)
		m.On("MustGet", stream.DBDep).Return(test.db)
		m.On("MustGet", stream.StorageDep).Return(test.storage)
		m.On("ParsePayload", &ss.SessionRunCommand{}).Return(nil).Run(func(args mock.Arguments) {
			payload := args.Get(0).(*ss.SessionRunCommand)
			payload.Backtest = test.backtest.Name
			payload.SessionID = test.session.Id
		})
		m.On("Call", mock.Anything, dependency.GetEngineKey).Return(nil, errors.New("not working"))
		err := command.SessionRun(context.TODO(), m)
		test.Require().Error(err)
		m.AssertCalled(test.T(), "ParsePayload", mock.Anything)
		m.AssertCalled(test.T(), "MustGet", stream.DBDep)
		m.AssertCalled(test.T(), "Call", mock.Anything, dependency.GetEngineKey)
	})
	test.Run("no ingestions", func() {
		m := new(stream.MockMessage)
		engine := new(engine.MockEngine)

		m.On("MustGet", stream.DBDep).Return(test.db)
		m.On("MustGet", stream.StorageDep).Return(test.storage)

		ingestions := []storage.Object{}
		test.storage.On("ListObjects", mock.Anything, storage.IngestionsBucket).Return(&ingestions, nil)
		m.On("ParsePayload", &ss.SessionRunCommand{}).Return(nil).Run(func(args mock.Arguments) {
			payload := args.Get(0).(*ss.SessionRunCommand)
			payload.Backtest = test.backtest.Name
			payload.SessionID = test.session.Id
		})
		m.On("Call", mock.Anything, dependency.GetEngineKey).Return(engine, nil)
		err := command.SessionRun(context.TODO(), m)
		test.Require().ErrorContains(err, "no ingestions found")
	})
	test.Run("successful", func() {
		m := new(stream.MockMessage)
		engine := new(engine.MockEngine)
		engine.On("DownloadIngestion", mock.Anything, mock.Anything).Return(nil)
		engine.On("Stop", mock.Anything).Return(nil)
		m.On("MustGet", stream.DBDep).Return(test.db)
		m.On("MustGet", stream.StorageDep).Return(test.storage)

		ingestions := []storage.Object{
			{
				LastModified: time.Now(),
				Metadata:     map[string]string{"Status": pb.IngestionStatus_READY.String()},
			},
		}
		test.storage.On("ListObjects", mock.Anything, storage.IngestionsBucket).Return(&ingestions, nil)
		m.On("ParsePayload", &ss.SessionRunCommand{}).Return(nil).Run(func(args mock.Arguments) {
			payload := args.Get(0).(*ss.SessionRunCommand)
			payload.Backtest = test.backtest.Name
			payload.SessionID = test.session.Id
		})
		m.On("Call", mock.Anything, dependency.GetEngineKey).Return(engine, nil)
		err := command.SessionRun(context.TODO(), m)
		test.Require().NoError(err)
		m.AssertCalled(test.T(), "ParsePayload", mock.Anything)
		m.AssertCalled(test.T(), "MustGet", stream.DBDep)
		m.AssertCalled(test.T(), "Call", mock.Anything, dependency.GetEngineKey)
		time.Sleep(time.Second / 2) // Wait for the session to start

		sessions := repository.Session{Conn: test.db}
		session, err := sessions.Get(context.TODO(), test.session.Id)
		test.Require().NoError(err)
		test.Require().NotNil(session.Port)
		test.Equal(pb.Session_Status_RUNNING, session.Statuses[0].Status)

		conn, err := grpc.NewClient(
			fmt.Sprintf("localhost:%d", *session.Port),
			grpc.WithTransportCredentials(
				insecure.NewCredentials(),
			),
		)
		test.Require().NoError(err)

		client := pb.NewSessionServicerClient(conn)
		rsp, err := client.StopServer(context.TODO(), &pb.StopServerRequest{})
		test.Require().NoError(err)
		test.NotNil(rsp)

		time.Sleep(time.Second) // Wait for the session to stop

		session, err = sessions.Get(context.TODO(), test.session.Id)
		test.Require().NoError(err)
		test.Equal(pb.Session_Status_COMPLETED, session.Statuses[0].Status)
	})
}
