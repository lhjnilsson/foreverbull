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

type StrategyTest struct {
	suite.Suite

	conn *pgxpool.Pool
}

func (test *StrategyTest) SetupSuite() {
	test_helper.SetupEnvironment(test.T(), &test_helper.Containers{
		Postgres: true,
	})
}

func (test *StrategyTest) SetupTest() {
	var err error
	test.conn, err = pgxpool.New(context.Background(), environment.GetPostgresURL())
	test.Require().NoError(err)

	err = repository.Recreate(context.Background(), test.conn)
	test.Require().NoError(err)
}

func TestStrategy(t *testing.T) {
	suite.Run(t, new(StrategyTest))
}

func (test *StrategyTest) TestCreate() {
	ctx := context.Background()
	strategies := repository.Strategy{Conn: test.conn}

	description := "mean reversion"
	strategy, err := strategies.Create(ctx, "mean-rev", &description)
	test.Require().NoError(err)
	test.NotEmpty(strategy.Id)
	test.Equal("mean-rev", strategy.Name)
	test.Equal(description, strategy.GetDescription())
	test.NotNil(strategy.CreatedAt)
}

func (test *StrategyTest) TestCreateWithoutDescription() {
	ctx := context.Background()
	strategies := repository.Strategy{Conn: test.conn}

	strategy, err := strategies.Create(ctx, "mean-rev", nil)
	test.Require().NoError(err)
	test.Nil(strategy.Description)
}

func (test *StrategyTest) TestCreateUniqueName() {
	ctx := context.Background()
	strategies := repository.Strategy{Conn: test.conn}

	_, err := strategies.Create(ctx, "mean-rev", nil)
	test.Require().NoError(err)

	_, err = strategies.Create(ctx, "mean-rev", nil)
	test.Require().Error(err)
}

func (test *StrategyTest) TestGet() {
	ctx := context.Background()
	strategies := repository.Strategy{Conn: test.conn}

	created, err := strategies.Create(ctx, "mean-rev", nil)
	test.Require().NoError(err)

	got, err := strategies.Get(ctx, "mean-rev")
	test.Require().NoError(err)
	test.Equal(created.Id, got.Id)

	byID, err := strategies.GetByID(ctx, created.Id)
	test.Require().NoError(err)
	test.Equal(created.Name, byID.Name)
}

func (test *StrategyTest) TestGetMissing() {
	ctx := context.Background()
	strategies := repository.Strategy{Conn: test.conn}

	got, err := strategies.Get(ctx, "missing")
	test.Require().Error(err)
	test.Nil(got)
}

func (test *StrategyTest) TestList() {
	ctx := context.Background()
	strategies := repository.Strategy{Conn: test.conn}

	_, err := strategies.Create(ctx, "a", nil)
	test.Require().NoError(err)
	_, err = strategies.Create(ctx, "b", nil)
	test.Require().NoError(err)

	listed, err := strategies.List(ctx)
	test.Require().NoError(err)
	test.Len(listed, 2)
}

func (test *StrategyTest) TestDelete() {
	ctx := context.Background()
	strategies := repository.Strategy{Conn: test.conn}

	_, err := strategies.Create(ctx, "mean-rev", nil)
	test.Require().NoError(err)

	err = strategies.Delete(ctx, "mean-rev")
	test.Require().NoError(err)

	got, err := strategies.Get(ctx, "mean-rev")
	test.Require().Error(err)
	test.Nil(got)
}

func (test *StrategyTest) TestDeleteCascadesChildren() {
	ctx := context.Background()

	_, version, err := createStrategyAndVersion(ctx, test.conn, "mean-rev", "algo:local", nil)
	test.Require().NoError(err)

	experiments := repository.Experiment{Conn: test.conn}
	_, err = experiments.Create(ctx, version.Id, "spy")
	test.Require().NoError(err)

	strategies := repository.Strategy{Conn: test.conn}
	err = strategies.Delete(ctx, "mean-rev")
	test.Require().NoError(err)

	listed, err := experiments.List(ctx, "", "", "")
	test.Require().NoError(err)
	test.Empty(listed)
}
