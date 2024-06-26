package command

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/internal/stream"
	"github.com/lhjnilsson/foreverbull/internal/test_helper"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/engine"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/repository"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/stream/dependency"
	bs "github.com/lhjnilsson/foreverbull/pkg/backtest/stream"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type CommandIngestTest struct {
	suite.Suite

	db *pgxpool.Pool
}

func (test *CommandIngestTest) SetupSuite() {
	var err error
	test_helper.SetupEnvironment(test.T(), &test_helper.Containers{
		Postgres: true,
	})
	test.db, err = pgxpool.New(context.Background(), environment.GetPostgresURL())
	test.Require().NoError(err)
}

func (test *CommandIngestTest) SetupTest() {
	test.Require().NoError(repository.Recreate(context.Background(), test.db))
}

func TestCommandIngest(t *testing.T) {
	suite.Run(t, new(CommandIngestTest))
}

func (test *CommandIngestTest) TestUpdateIngestStatus() {
	m := new(stream.MockMessage)
	m.On("MustGet", stream.DBDep).Return(test.db)

	ingestions := repository.Ingestion{Conn: test.db}
	_, err := ingestions.Create(context.Background(), "test-ingestion", time.Now(), time.Now(), "test-calendar", []string{})
	test.NoError(err)

	m.On("ParsePayload", mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		command := args.Get(0).(*bs.UpdateIngestStatusCommand)
		command.Name = "test-ingestion"
		command.Status = "test-status"
	})

	ctx := context.Background()
	err = UpdateIngestStatus(ctx, m)
	test.NoError(err)
}

func (test *CommandIngestTest) TestIngestCommand() {
	ingestions := repository.Ingestion{Conn: test.db}
	_, err := ingestions.Create(context.Background(), "test-ingestion", time.Now(), time.Now(), "test-calendar", []string{})
	test.NoError(err)

	m := new(stream.MockMessage)
	m.On("MustGet", stream.DBDep).Return(test.db)

	backtest := new(engine.MockEngine)
	m.On("Call", mock.Anything, dependency.GetIngestEngineKey).Return(backtest, nil)
	backtest.On("Ingest", mock.Anything, mock.Anything).Return(nil)
	backtest.On("UploadIngestion", mock.Anything, mock.Anything).Return(nil)

	m.On("ParsePayload", mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		command := args.Get(0).(*bs.IngestCommand)
		command.Name = "test-ingestion"
		command.ServiceInstanceID = "test-instance"
	})

	ctx := context.Background()
	err = Ingest(ctx, m)
	test.NoError(err)
}
