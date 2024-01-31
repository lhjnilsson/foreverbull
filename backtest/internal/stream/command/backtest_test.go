package command

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/lhjnilsson/foreverbull/backtest/entity"
	bs "github.com/lhjnilsson/foreverbull/backtest/stream"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/backtest/internal/repository"
	"github.com/lhjnilsson/foreverbull/backtest/internal/stream/dependency"
	"github.com/lhjnilsson/foreverbull/internal/config"
	"github.com/lhjnilsson/foreverbull/internal/stream"
	"github.com/lhjnilsson/foreverbull/tests/helper"
	mockStream "github.com/lhjnilsson/foreverbull/tests/mocks/internal_/stream"
	mockEngine "github.com/lhjnilsson/foreverbull/tests/mocks/service/backtest/engine"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
)

type CommandBacktestTest struct {
	suite.Suite

	log    *zap.Logger
	db     *pgxpool.Pool
	config *config.Config
}

func TestCommandBacktest(t *testing.T) {
	suite.Run(t, new(CommandBacktestTest))
}

func (test *CommandBacktestTest) SetupTest() {
	test.config = helper.TestingConfig(test.T(), &helper.Containers{
		Postgres: true,
	})

	test.log = zaptest.NewLogger(test.T())

	var err error
	test.db, err = pgxpool.New(context.Background(), test.config.PostgresURI)
	test.NoError(err)

	err = repository.Recreate(context.TODO(), test.db)
	test.NoError(err)
}

func (test *CommandBacktestTest) SetupSubTest() {
}

func (test *CommandBacktestTest) TearDownTest() {
}

func (test *CommandBacktestTest) TestBacktestUpdateStatusCommand() {
	m := new(mockStream.Message)
	m.On("MustGet", stream.DBDep).Return(test.db)

	backtests := repository.Backtest{Conn: test.db}
	_, err := backtests.Create(context.Background(), "test-backtest", "test-backtest-service", nil, time.Now(), time.Now(),
		"test-calendar", []string{"test-symbol"}, nil)
	test.NoError(err)

	type TestCase struct {
		Status entity.BacktestStatusType
		Error  error
	}
	testCases := []TestCase{
		{
			Status: entity.BacktestStatusIngesting,
			Error:  nil,
		},
		{
			Status: entity.BacktestStatusReady,
			Error:  nil,
		},
		{
			Status: entity.BacktestStatusError,
			Error:  errors.New("test-error"),
		},
	}
	for _, tc := range testCases {
		test.Run(string(tc.Status), func() {
			m.On("ParsePayload", mock.Anything).Return(nil).Run(func(args mock.Arguments) {
				command := args.Get(0).(*bs.UpdateBacktestStatusCommand)
				command.BacktestName = "test-backtest"
				command.Status = tc.Status
				command.Error = tc.Error
			})

			ctx := context.Background()
			err := UpdateBacktestStatus(ctx, m)
			test.NoError(err)

			backtest, err := backtests.Get(ctx, "test-backtest")
			test.NoError(err)
			test.Equal(tc.Status, backtest.Statuses[0].Status)
			if tc.Error != nil {
				test.Equal(tc.Error.Error(), *backtest.Statuses[0].Error)
			} else {
				test.Nil(backtest.Statuses[0].Error)
			}
		})
	}
}

func (test *CommandBacktestTest) TestBacktestIngestCommand() {
	backtests := repository.Backtest{Conn: test.db}
	_, err := backtests.Create(context.Background(), "test-backtest", "test-backtest-service", nil, time.Now(), time.Now(),
		"test-calendar", []string{"test-symbol"}, nil)
	test.NoError(err)

	m := new(mockStream.Message)
	m.On("MustGet", stream.DBDep).Return(test.db)
	m.On("MustGet", stream.LoggerDep).Return(test.log)
	m.On("MustGet", stream.ConfigDep).Return(test.config)

	engine := new(mockEngine.Engine)
	m.On("Call", mock.Anything, dependency.GetBacktestEngineKey).Return(engine, nil)
	engine.On("Ingest", mock.Anything, mock.Anything).Return(nil)
	engine.On("UploadIngestion", mock.Anything, mock.Anything).Return(nil)

	m.On("ParsePayload", mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		command := args.Get(0).(*bs.BacktestIngestCommand)
		command.BacktestName = "test-backtest"
		command.ServiceInstanceID = "test-instance"
	})

	ctx := context.Background()
	err = BacktestIngest(ctx, m)
	test.NoError(err)
}
