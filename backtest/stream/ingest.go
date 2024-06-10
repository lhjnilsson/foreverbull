package stream

import (
	"github.com/lhjnilsson/foreverbull/backtest/entity"
	financeStream "github.com/lhjnilsson/foreverbull/finance/stream"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/internal/stream"
	serviceStream "github.com/lhjnilsson/foreverbull/service/stream"
)

type UpdateIngestStatusCommand struct {
	Name   string                     `json:"name"`
	Status entity.IngestionStatusType `json:"status"`
	Error  error                      `json:"error"`
}

func NewUpdateIngestStatusCommand(name string, status entity.IngestionStatusType, err error) (stream.Message, error) {
	entity := &UpdateIngestStatusCommand{
		Name:   name,
		Status: status,
		Error:  err,
	}
	return stream.NewMessage("backtest", "ingest", "status", entity)
}

type IngestCommand struct {
	Name              string `json:"backtest_name"`
	ServiceInstanceID string `json:"service_instance_id"`
}

func NewBacktestIngestCommand(name, serviceInstanceID string) (stream.Message, error) {
	entity := &IngestCommand{
		Name:              name,
		ServiceInstanceID: serviceInstanceID,
	}
	return stream.NewMessage("backtest", "ingest", "ingest", entity)
}

func NewIngestOrchestration(ingest *entity.Ingestion) (*stream.MessageOrchestration, error) {
	orchestration := stream.NewMessageOrchestration("ingest backtest")

	backtestInstanceID := serviceStream.NewInstanceID()
	msg, err := serviceStream.NewServiceStartCommand(environment.GetBacktestImage(), backtestInstanceID)
	if err != nil {
		return nil, err
	}
	msg2, err := NewUpdateBacktestStatusCommand(ingest.Name, entity.BacktestStatusIngesting, nil)
	if err != nil {
		return nil, err
	}
	orchestration.AddStep("start service", []stream.Message{msg, msg2})

	msg, err = serviceStream.NewInstanceSanityCheckCommand([]string{backtestInstanceID})
	if err != nil {
		return nil, err
	}
	msg2, err = financeStream.NewIngestCommand(ingest.Symbols, ingest.Start, ingest.End)
	if err != nil {
		return nil, err
	}
	orchestration.AddStep("sanity check", []stream.Message{msg, msg2})

	msg, err = NewBacktestIngestCommand(ingest.Name, backtestInstanceID)
	if err != nil {
		return nil, err
	}
	orchestration.AddStep("ingest backtest", []stream.Message{msg})

	msg, err = serviceStream.NewInstanceStopCommand(backtestInstanceID)
	if err != nil {
		return nil, err
	}
	msg2, err = NewUpdateBacktestStatusCommand(ingest.Name, entity.BacktestStatusReady, nil)
	if err != nil {
		return nil, err
	}
	orchestration.AddStep("stop service", []stream.Message{msg, msg2})

	msg, err = serviceStream.NewInstanceStopCommand(backtestInstanceID)
	if err != nil {
		return nil, err
	}
	msg2, err = NewUpdateBacktestStatusCommand(ingest.Name, entity.BacktestStatusError, nil)
	if err != nil {
		return nil, err
	}
	orchestration.SettFallback([]stream.Message{msg, msg2})

	return orchestration, nil
}
