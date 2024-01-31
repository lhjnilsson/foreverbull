package repository

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/strategy/entity"
	"github.com/lhjnilsson/foreverbull/tests/helper"
	"github.com/stretchr/testify/assert"
)

func TestStrategy(t *testing.T) {
	var err error
	config := helper.TestingConfig(t, &helper.Containers{
		Postgres: true,
	})
	conn, err := pgxpool.New(context.Background(), config.PostgresURI)
	assert.Nil(t, err)
	defer conn.Close()

	conn.Exec(context.Background(), "DROP TABLE strategy")
	_, err = conn.Exec(context.Background(), StrategyTable)
	assert.Nil(t, err)

	store := Strategy{Conn: conn}

	entries, err := store.List(context.Background())
	assert.Nil(t, err)
	assert.Empty(t, entries)

	name := "TEST_STRATEGY"
	strategy := entity.Strategy{Name: &name}
	err = store.Create(context.Background(), &strategy)
	assert.Nil(t, err)
	assert.NotEmpty(t, strategy.CreatedAt)

	entries, err = store.List(context.Background())
	assert.Nil(t, err)
	assert.NotEmpty(t, entries)

	read, err := store.Get(context.Background(), "TEST_STRATEGY")
	assert.Nil(t, err)
	assert.Equal(t, "TEST_STRATEGY", *read.Name)

	err = store.SetBacktest(context.Background(), *strategy.Name, "backtest")
	assert.Nil(t, err)

	err = store.Delete(context.Background(), "TEST_STRATEGY")
	assert.Nil(t, err)
}
