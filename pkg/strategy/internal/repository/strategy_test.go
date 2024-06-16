package repository

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/internal/test_helper"
	"github.com/lhjnilsson/foreverbull/pkg/strategy/entity"
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
	conn, err := pgxpool.New(context.Background(), environment.GetPostgresURL())
	test.Require().NoError(err)
	test.conn = conn
}

func (test *StrategyTest) SetupTest() {
	test.Require().NoError(Recreate(context.Background(), test.conn))
}

func TestStrategy(t *testing.T) {
	suite.Run(t, new(StrategyTest))
}

func (test *StrategyTest) TestCreate() {
	ctx := context.Background()

	db := &Strategy{Conn: test.conn}

	strategy, err := db.Create(ctx, "test", []string{"AAPL"}, 10, "worker")
	test.Require().NoError(err)
	test.Equal(&entity.Strategy{
		Name:      "test",
		Symbols:   []string{"AAPL"},
		MinDays:   10,
		Service:   "worker",
		CreatedAt: strategy.CreatedAt,
	}, strategy)
}

func (test *StrategyTest) TestList() {
	ctx := context.Background()

	db := &Strategy{Conn: test.conn}

	strategy, err := db.Create(ctx, "test", []string{"AAPL"}, 10, "worker")
	test.Require().NoError(err)

	strategies, err := db.List(ctx)
	test.Require().NoError(err)
	test.Equal([]entity.Strategy{*strategy}, *strategies)
}

func (test *StrategyTest) TestGet() {
	ctx := context.Background()

	db := &Strategy{Conn: test.conn}

	strategy, err := db.Create(ctx, "test", []string{"AAPL"}, 10, "worker")
	test.Require().NoError(err)

	s, err := db.Get(ctx, "test")
	test.Require().NoError(err)
	test.Equal(strategy, s)
}

func (test *StrategyTest) TestDelete() {
	ctx := context.Background()

	db := &Strategy{Conn: test.conn}

	_, err := db.Create(ctx, "test", []string{"AAPL"}, 10, "worker")
	test.Require().NoError(err)

	err = db.Delete(ctx, "test")
	test.Require().NoError(err)

	s, err := db.Get(ctx, "test")
	test.Require().Error(err)
	test.Nil(s)
}
