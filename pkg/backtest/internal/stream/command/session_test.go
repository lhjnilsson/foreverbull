package command

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/lhjnilsson/foreverbull/pkg/backtest/entity"
	ss "github.com/lhjnilsson/foreverbull/pkg/backtest/stream"
	"github.com/lhjnilsson/foreverbull/pkg/service/socket"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/internal/stream"
	"github.com/lhjnilsson/foreverbull/internal/test_helper"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/backtest"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/repository"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/stream/dependency"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type CommandSessionTest struct {
	suite.Suite

	db *pgxpool.Pool
}

func TestCommandSession(t *testing.T) {
	suite.Run(t, new(CommandSessionTest))
}

func (test *CommandSessionTest) SetupTest() {
	test_helper.SetupEnvironment(test.T(), &test_helper.Containers{
		Postgres: true,
	})

	var err error
	test.db, err = pgxpool.New(context.Background(), environment.GetPostgresURL())
	test.Require().NoError(err)

	err = repository.Recreate(context.TODO(), test.db)
	test.Require().NoError(err)
}

func (test *CommandSessionTest) TearDownTest() {
}

func (test *CommandSessionTest) TestUpdateSessionCommand() {

	backtests := repository.Backtest{Conn: test.db}
	backtest, err := backtests.Create(context.Background(), "test-backtest", nil, time.Now(), time.Now(),
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
		m := new(stream.MockMessage)
		m.On("MustGet", stream.DBDep).Return(test.db)
		m.On("ParsePayload", mock.Anything).Return(nil).Run(func(args mock.Arguments) {
			command := args.Get(0).(*ss.UpdateSessionStatusCommand)
			command.SessionID = session.ID
			command.Status = tc.Status
			command.Error = tc.Error
		})

		err := UpdateSessionStatus(context.Background(), m)
		test.NoError(err)

		session, err := sessions.Get(context.Background(), session.ID)
		test.NoError(err)
		test.Equal(tc.Status, session.Statuses[0].Status)
		if tc.Error != nil {
			test.Require().NotNil(session.Statuses[0].Error)
			test.Equal(tc.Error.Error(), *session.Statuses[0].Error)
		} else {
			test.Nil(session.Statuses[0].Error)
		}
	}
}

func (test *CommandSessionTest) TestSessionRunCommand() {
	backtests := repository.Backtest{Conn: test.db}
	sessions := repository.Session{Conn: test.db}
	_, err := backtests.Create(context.Background(), "test-backtest", nil, time.Now(), time.Now(),
		"test-calendar", []string{"test-symbol"}, nil)
	test.NoError(err)
	s, err := sessions.Create(context.Background(), "test-backtest", false)
	test.NoError(err)

	m := new(stream.MockMessage)
	m.On("MustGet", stream.DBDep).Return(test.db)

	session := new(backtest.MockSession)
	session.On("GetSocket").Return(&socket.Socket{})
	session.On("Run", mock.Anything, mock.Anything).Return(nil)
	session.On("Stop", mock.Anything).Return(nil)

	m.On("Call", mock.Anything, dependency.GetBacktestSessionKey).Return(session, nil)
	m.On("ParsePayload", mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		command := args.Get(0).(*ss.SessionRunCommand)
		command.SessionID = s.ID
	})

	err = SessionRun(context.Background(), m)
	test.NoError(err)
}
