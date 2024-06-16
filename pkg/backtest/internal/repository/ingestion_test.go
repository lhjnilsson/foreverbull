package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/entity"
	"github.com/lhjnilsson/foreverbull/tests/helper"
	"github.com/stretchr/testify/suite"
)

type IngestionTest struct {
	suite.Suite

	conn *pgxpool.Pool
}

func (test *IngestionTest) SetupSuite() {
	var err error
	helper.SetupEnvironment(test.T(), &helper.Containers{
		Postgres: true,
	})
	test.conn, err = pgxpool.New(context.Background(), environment.GetPostgresURL())
	test.Require().NoError(err)
}

func (test *IngestionTest) SetupTest() {
	err := Recreate(context.Background(), test.conn)
	test.Require().NoError(err)
}

func (test *IngestionTest) TearDownTest() {
}

func TestIngestions(t *testing.T) {
	suite.Run(t, new(IngestionTest))
}

func (test *IngestionTest) TestIngestion() {
	ctx := context.Background()

	db := &Ingestion{Conn: test.conn}
	var ingestion *entity.Ingestion
	var err error

	test.Run("Create", func() {
		ingestion, err = db.Create(ctx, "demo", time.Now(), time.Now(), "XNYS", []string{})
		test.NoError(err)
		test.Equal("XNYS", ingestion.Calendar)
		test.Len(ingestion.Statuses, 1)
	})
	test.Run("Update status", func() {
		err := db.UpdateStatus(ctx, ingestion.Name, "ERROR", errors.New("test error"))
		test.NoError(err)
		ingestion, err = db.Get(ctx, ingestion.Name)
		test.NoError(err)
		test.Equal(entity.IngestionStatusError, ingestion.Statuses[0].Status)
		test.Equal("test error", *ingestion.Statuses[0].Error)
	})
	test.Run("Get", func() {
		ingestion, err = db.Get(ctx, ingestion.Name)
		test.NoError(err)
		test.Equal("demo", ingestion.Name)

		test.Len(ingestion.Statuses, 2)
		test.Equal(entity.IngestionStatusError, ingestion.Statuses[0].Status)
	})
}
