package api

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/backtest/entity"
	"github.com/lhjnilsson/foreverbull/backtest/internal/repository"
	"github.com/stretchr/testify/assert"
)

func AddBacktest(t *testing.T, conn *pgxpool.Pool, name string) *entity.Backtest {
	repository_b := repository.Backtest{Conn: conn}
	start, err := time.Parse("2006-01-02", "2020-01-01")
	assert.Nil(t, err)
	end, err := time.Parse("2006-01-02", "2020-12-31")
	assert.Nil(t, err)

	worker := "worker_service"
	backtest, err := repository_b.Create(context.Background(), name, "backtest_service", &worker, start, end, "XNYS", []string{"AAPL"}, nil)
	assert.Nil(t, err)
	err = repository_b.UpdateStatus(context.Background(), name, entity.BacktestStatusReady, nil)
	assert.Nil(t, err)
	return backtest
}

func AddSession(t *testing.T, conn *pgxpool.Pool, backtest string) *entity.Session {
	repository_s := repository.Session{Conn: conn}
	session, err := repository_s.Create(context.Background(), backtest, false)
	assert.Nil(t, err)
	return session
}

func AddExecution(t *testing.T, conn *pgxpool.Pool, sessionID string) *entity.Execution {
	repository_e := repository.Execution{Conn: conn}

	execution, err := repository_e.Create(context.TODO(), sessionID, "XNYS", time.Now(), time.Now(),
		[]string{}, nil)
	assert.Nil(t, err)
	return execution
}
