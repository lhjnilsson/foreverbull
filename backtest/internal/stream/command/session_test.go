package command

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/lhjnilsson/foreverbull/backtest/entity"
	ss "github.com/lhjnilsson/foreverbull/backtest/stream"
	"github.com/lhjnilsson/foreverbull/service/socket"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/backtest/internal/repository"
	"github.com/lhjnilsson/foreverbull/backtest/internal/stream/dependency"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/internal/stream"
	"github.com/lhjnilsson/foreverbull/tests/helper"
	mockBacktest "github.com/lhjnilsson/foreverbull/tests/mocks/backtest/internal_/backtest"
	mockStream "github.com/lhjnilsson/foreverbull/tests/mocks/internal_/stream"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
)

type CommandSessionTest struct {
	suite.Suite

	log *zap.Logger
	db  *pgxpool.Pool
}

func TestCommandSession(t *testing.T) {
	suite.Run(t, new(CommandSessionTest))
}

func (test *CommandSessionTest) SetupTest() {
	helper.SetupEnvironment(test.T(), &helper.Containers{
		Postgres: true,
	})

	test.log = zaptest.NewLogger(test.T())

	var err error
	test.db, err = pgxpool.New(context.Background(), environment.GetPostgresURL())
	test.NoError(err)

	err = repository.Recreate(context.TODO(), test.db)
	test.NoError(err)
}

func (test *CommandSessionTest) TearDownTest() {
}

func (test *CommandSessionTest) TestUpdateSessionCommand() {
	m := new(mockStream.Message)
	m.On("MustGet", stream.DBDep).Return(test.db)

	backtests := repository.Backtest{Conn: test.db}
	backtest, err := backtests.Create(context.Background(), "test-backtest", "test-backtest-service", nil, time.Now(), time.Now(),
		"test-calendar", []string{"test-symbol"}, nil)
	test.NoError(err)

	sessions := repository.Session{Conn: test.db}
	session, err := sessions.Create(context.Background(), backtest.Name, false)
	test.NoError(err)

	type TestCase struct {
		Status entity.SessionStatusType
		Error  error
	}
	testCases := []TestCase{
		{Status: entity.SessionStatusRunning, Error: nil},
		{Status: entity.SessionStatusCompleted, Error: nil},
		{Status: entity.SessionStatusFailed, Error: errors.New("test error")},
	}

	for _, tc := range testCases {
		m.On("ParsePayload", mock.Anything).Return(nil).Run(func(args mock.Arguments) {
			command := args.Get(0).(*ss.UpdateSessionStatusCommand)
			command.SessionID = session.ID
			command.Status = tc.Status
			command.Error = tc.Error
		})

		err := UpdateSessionStatus(context.Background(), m)
		test.NoError(err)

		sessions := repository.Session{Conn: test.db}
		session, err := sessions.Get(context.Background(), session.ID)
		test.NoError(err)
		test.Equal(tc.Status, session.Statuses[0].Status)
		if tc.Error != nil {
			test.Equal(tc.Error.Error(), *session.Statuses[0].Error)
		} else {
			test.Nil(session.Statuses[0].Error)
		}
	}
}

func (test *CommandSessionTest) TestSessionRunCommand() {
	backtests := repository.Backtest{Conn: test.db}
	sessions := repository.Session{Conn: test.db}
	_, err := backtests.Create(context.Background(), "test-backtest", "test-backtest-service", nil, time.Now(), time.Now(),
		"test-calendar", []string{"test-symbol"}, nil)
	test.NoError(err)
	s, err := sessions.Create(context.Background(), "test-backtest", false)
	test.NoError(err)

	m := new(mockStream.Message)
	m.On("MustGet", stream.DBDep).Return(test.db)
	m.On("MustGet", stream.LoggerDep).Return(test.log)

	session := new(mockBacktest.Session)
	session.On("GetSocket").Return(&socket.Socket{})
	session.On("Run", mock.Anything, mock.Anything).Return(nil)

	m.On("Call", mock.Anything, dependency.GetBacktestSessionKey).Return(session, nil)
	m.On("ParsePayload", mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		command := args.Get(0).(*ss.SessionRunCommand)
		command.SessionID = s.ID
	})

	err = SessionRun(context.Background(), m)
	test.NoError(err)
}
