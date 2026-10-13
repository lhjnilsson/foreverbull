package repository_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/internal/test_helper"
	"github.com/lhjnilsson/foreverbull/pkg/strategy/internal/repository"
	"github.com/stretchr/testify/suite"
)

type StorageTest struct {
	suite.Suite

	conn *pgxpool.Pool
}

func (test *StorageTest) SetupSuite() {
	test_helper.SetupEnvironment(test.T(), &test_helper.Containers{
		Postgres: true,
	})
}

func (test *StorageTest) SetupTest() {
	var err error
	test.conn, err = pgxpool.New(context.Background(), environment.GetPostgresURL())
	test.Require().NoError(err)

	err = repository.Recreate(context.Background(), test.conn)
	test.Require().NoError(err)
}

func TestStorage(t *testing.T) {
	suite.Run(t, new(StorageTest))
}

func (test *StorageTest) TestCreateTablesIdempotent() {
	ctx := context.Background()
	err := repository.CreateTables(ctx, test.conn)
	test.Require().NoError(err)
	err = repository.CreateTables(ctx, test.conn)
	test.Require().NoError(err)
}

func (test *StorageTest) TestRecreateResetsRows() {
	ctx := context.Background()

	strategies := repository.Strategy{Conn: test.conn}
	_, err := strategies.Create(ctx, "mean-rev", nil)
	test.Require().NoError(err)

	err = repository.Recreate(ctx, test.conn)
	test.Require().NoError(err)

	listed, err := strategies.List(ctx)
	test.Require().NoError(err)
	test.Empty(listed)
}

func (test *StorageTest) TestParentInsertsOmitNewColumns() {
	ctx := context.Background()
	_, err := test.conn.Exec(ctx, `INSERT INTO backtest (name, benchmark) VALUES ('spy', 'SPY')`)
	test.Require().NoError(err)

	var provider, profile string
	err = test.conn.QueryRow(ctx, `SELECT provider, profile FROM backtest WHERE name='spy'`).Scan(&provider, &profile)
	test.Require().NoError(err)
	test.Equal("yahoo", provider)
	test.Empty(profile)

	var sessionID string
	err = test.conn.QueryRow(ctx, `INSERT INTO session (backtest) VALUES ('spy') RETURNING id`).Scan(&sessionID)
	test.Require().NoError(err)

	var executionID, executionProvider string
	var arguments []byte
	err = test.conn.QueryRow(ctx, `
		INSERT INTO backtest_execution (session, start_date, end_date)
		VALUES ($1, '2020-01-01', '2020-12-31')
		RETURNING id, provider, function_arguments`, sessionID).Scan(&executionID, &executionProvider, &arguments)
	test.Require().NoError(err)
	test.NotEmpty(executionID)
	test.Equal("yahoo", executionProvider)
	test.Empty(arguments)

	_, err = test.conn.Exec(ctx, `INSERT INTO backtest (name, benchmark, provider) VALUES ('nasdaq-bt', 'QQQ', 'nasdaq')`)
	test.Require().ErrorContains(err, "backtest")
}
