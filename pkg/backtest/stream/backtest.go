package stream

import (
	"github.com/lhjnilsson/foreverbull/internal/stream"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/entity"
)

type UpdateBacktestStatusCommand struct {
	BacktestName string                    `json:"backtest_name"`
	Status       entity.BacktestStatusType `json:"status"`
	Error        error                     `json:"error"`
}

func NewUpdateBacktestStatusCommand(backtest string, status entity.BacktestStatusType, err error) (stream.Message, error) {
	entity := &UpdateBacktestStatusCommand{
		BacktestName: backtest,
		Status:       status,
		Error:        err,
	}
	return stream.NewMessage("backtest", "backtest", "status", entity)
}
