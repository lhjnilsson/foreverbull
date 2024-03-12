package event

import (
	"github.com/lhjnilsson/foreverbull/backtest/entity"
	financeStream "github.com/lhjnilsson/foreverbull/finance/stream"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/internal/stream"
	serviceStream "github.com/lhjnilsson/foreverbull/service/stream"
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

type BacktestIngestCommand struct {
	BacktestName      string `json:"backtest_name"`
	ServiceInstanceID string `json:"service_instance_id"`
}

func NewBacktestIngestCommand(backtest, serviceInstanceID string) (stream.Message, error) {
	entity := &BacktestIngestCommand{
		BacktestName:      backtest,
		ServiceInstanceID: serviceInstanceID,
	}
	return stream.NewMessage("backtest", "backtest", "ingest", entity)
}

func NewBacktestIngestOrchestration(backtest *entity.Backtest) (*stream.MessageOrchestration, error) {
	orchestration := stream.NewMessageOrchestration("ingest backtest")

	backtestInstanceID := serviceStream.NewInstanceID()
	msg, err := serviceStream.NewServiceStartCommand(environment.GetBacktestImage(), backtestInstanceID)
	if err != nil {
		return nil, err
	}
	msg2, err := NewUpdateBacktestStatusCommand(backtest.Name, entity.BacktestStatusIngesting, nil)
	if err != nil {
		return nil, err
	}
	orchestration.AddStep("start service", []stream.Message{msg, msg2})

	msg, err = serviceStream.NewInstanceSanityCheckCommand([]string{backtestInstanceID})
	if err != nil {
		return nil, err
	}
	msg2, err = financeStream.NewIngestCommand(backtest.Symbols, backtest.Start, backtest.End)
	if err != nil {
		return nil, err
	}
	orchestration.AddStep("sanity check", []stream.Message{msg, msg2})

	msg, err = NewBacktestIngestCommand(backtest.Name, backtestInstanceID)
	if err != nil {
		return nil, err
	}
	orchestration.AddStep("ingest backtest", []stream.Message{msg})

	msg, err = serviceStream.NewInstanceStopCommand(backtestInstanceID)
	if err != nil {
		return nil, err
	}
	msg2, err = NewUpdateBacktestStatusCommand(backtest.Name, entity.BacktestStatusReady, nil)
	if err != nil {
		return nil, err
	}
	orchestration.AddStep("stop service", []stream.Message{msg, msg2})

	msg, err = serviceStream.NewInstanceStopCommand(backtestInstanceID)
	if err != nil {
		return nil, err
	}
	msg2, err = NewUpdateBacktestStatusCommand(backtest.Name, entity.BacktestStatusError, nil)
	if err != nil {
		return nil, err
	}
	orchestration.SettFallback([]stream.Message{msg, msg2})

	return orchestration, nil
}
