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
