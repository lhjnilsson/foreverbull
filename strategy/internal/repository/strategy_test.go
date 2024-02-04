package repository

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/strategy/entity"
	"github.com/lhjnilsson/foreverbull/tests/helper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type StrategyTestSuite struct {
	suite.Suite
	store *Strategy
	conn  *pgxpool.Pool
}

func (test *StrategyTestSuite) SetupTest() {
	helper.SetupEnvironment(test.T(), &helper.Containers{
		Postgres: true,
	})
	conn, err := pgxpool.New(context.Background(), environment.GetPostgresURL())
	assert.Nil(test.T(), err)
	test.conn = conn

	_, err = conn.Exec(context.Background(), "DROP TABLE IF EXISTS strategy")
	assert.Nil(test.T(), err)
	_, err = conn.Exec(context.Background(), StrategyTable)
	assert.Nil(test.T(), err)

	test.store = &Strategy{Conn: conn}
}

func (test *StrategyTestSuite) TearDownTest() {
	test.conn.Close()
}

func TestStrategySuite(t *testing.T) {
	suite.Run(t, new(StrategyTestSuite))
}

func (test *StrategyTestSuite) TestList() {
	entries, err := test.store.List(context.Background())
	assert.Nil(test.T(), err)
	assert.Empty(test.T(), entries)
}

func (test *StrategyTestSuite) TestCreateAndGet() {
	name := "TEST_STRATEGY"
	strategy := entity.Strategy{Name: &name}
	err := test.store.Create(context.Background(), &strategy)
	assert.Nil(test.T(), err)
	assert.NotEmpty(test.T(), strategy.CreatedAt)

	entries, err := test.store.List(context.Background())
	assert.Nil(test.T(), err)
	assert.NotEmpty(test.T(), entries)

	read, err := test.store.Get(context.Background(), "TEST_STRATEGY")
	assert.Nil(test.T(), err)
	assert.Equal(test.T(), "TEST_STRATEGY", *read.Name)
}

func (test *StrategyTestSuite) TestSetBacktest() {
	name := "TEST_STRATEGY"
	strategy := entity.Strategy{Name: &name}
	err := test.store.Create(context.Background(), &strategy)
	assert.Nil(test.T(), err)

	err = test.store.SetBacktest(context.Background(), *strategy.Name, "backtest")
	assert.Nil(test.T(), err)
}

func (test *StrategyTestSuite) TestDelete() {
	name := "TEST_STRATEGY"
	strategy := entity.Strategy{Name: &name}
	err := test.store.Create(context.Background(), &strategy)
	assert.Nil(test.T(), err)

	err = test.store.Delete(context.Background(), "TEST_STRATEGY")
	assert.Nil(test.T(), err)
}
