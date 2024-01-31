package event

import (
	"context"
	"fmt"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/config"
	"github.com/lhjnilsson/foreverbull/internal/stream"
	"github.com/lhjnilsson/foreverbull/strategy/entity"
	"github.com/lhjnilsson/foreverbull/strategy/internal/repository"
	"go.uber.org/zap"
)

type ExecutionCreated struct {
	Log    *zap.Logger
	Config *config.Config
	DBConn *pgxpool.Pool
	Stream stream.Stream
}

func (h *ExecutionCreated) Process(ctx context.Context, message stream.Message) error {
	e := entity.Execution{}
	err := message.ParsePayload(&e)
	if err != nil {
		h.Log.Error("Error unmarshalling ExecutionCreated payload", zap.Error(err))
		return err
	}

	strategies := repository.Strategy{Conn: h.DBConn}

	if err != nil {
		h.Log.Error("Error getting workflow", zap.Error(err))
		return err
	}

	strategy, err := strategies.Get(ctx, *e.Strategy)
	if err != nil {
		h.Log.Error("Error getting strategy", zap.Error(err))
		return err
	}

	if strategy == nil {
		h.Log.Error("Strategy not found", zap.Any("strategy", e))
		return fmt.Errorf("Strategy not found")
	}

	req, err := http.NewRequest("POST", "http://localhost:8080/finance/api/ohlc", nil)
	if err != nil {
		h.Log.Error("Error creating request", zap.Error(err))
		return err
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		h.Log.Error("Error sending request", zap.Error(err))
		return err
	}
	if res.StatusCode != 200 {
		h.Log.Error("Error updating OHLC", zap.Any("response", res.Status))
		return err
	}

	req, err = http.NewRequest("PATCH", "http://localhost:8080/backtest/api/backtests/"+*strategy.Backtest, res.Body)
	if err != nil {
		h.Log.Error("Error creating request", zap.Error(err))
		return err
	}
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		h.Log.Error("Error sending request", zap.Error(err))
		return err
	}
	if res.StatusCode != 200 {
		h.Log.Error("Error updating backtest", zap.Any("response", res.Status))
		return err
	}

	// Trigger backtest ingestion
	req, err = http.NewRequest("PUT", fmt.Sprintf("http://localhost:8080/backtest/api/backtests/%s/ingest", *strategy.Backtest), nil)
	if err != nil {
		h.Log.Error("Error creating request", zap.Error(err))
		return err
	}
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		h.Log.Error("Error sending request", zap.Error(err))
		return err
	}

	h.Log.Info("Successfully processed ExecutionCreated event", zap.Any("strategy", e), zap.Any("execution", e))
	return nil
}
