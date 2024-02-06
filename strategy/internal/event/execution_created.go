package event

import (
	"context"
	"fmt"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/stream"
	"github.com/lhjnilsson/foreverbull/strategy/entity"
	"github.com/lhjnilsson/foreverbull/strategy/internal/repository"
)

type ExecutionCreated struct {
	DBConn *pgxpool.Pool
	Stream stream.Stream
}

func (h *ExecutionCreated) Process(ctx context.Context, message stream.Message) error {
	e := entity.Execution{}
	err := message.ParsePayload(&e)
	if err != nil {
		return err
	}

	strategies := repository.Strategy{Conn: h.DBConn}

	if err != nil {
		return err
	}

	strategy, err := strategies.Get(ctx, *e.Strategy)
	if err != nil {
		return err
	}

	if strategy == nil {
		return fmt.Errorf("Strategy not found")
	}

	req, err := http.NewRequest("POST", "http://localhost:8080/finance/api/ohlc", nil)
	if err != nil {
		return err
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	if res.StatusCode != 200 {
		return err
	}

	req, err = http.NewRequest("PATCH", "http://localhost:8080/backtest/api/backtests/"+*strategy.Backtest, res.Body)
	if err != nil {
		return err
	}
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	if res.StatusCode != 200 {
		return err
	}

	// Trigger backtest ingestion
	req, err = http.NewRequest("PUT", fmt.Sprintf("http://localhost:8080/backtest/api/backtests/%s/ingest", *strategy.Backtest), nil)
	if err != nil {
		return err
	}
	_, err = http.DefaultClient.Do(req)
	if err != nil {
		return err
	}

	return nil
}
