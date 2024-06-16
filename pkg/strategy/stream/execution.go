package stream

import (
	"time"

	"github.com/lhjnilsson/foreverbull/internal/stream"
	financeStream "github.com/lhjnilsson/foreverbull/pkg/finance/stream"
	serviceStream "github.com/lhjnilsson/foreverbull/pkg/service/stream"
	"github.com/lhjnilsson/foreverbull/pkg/strategy/entity"
)

type UpdateExecutionStatusCommand struct {
	ExecutionID string                     `json:"execution_id"`
	Status      entity.ExecutionStatusType `json:"status"`
	Error       error                      `json:"error"`
}

func NewUpdateExecutionStatusCommand(executionID string, status entity.ExecutionStatusType, err error) (stream.Message, error) {
	entity := &UpdateExecutionStatusCommand{
		ExecutionID: executionID,
		Status:      status,
		Error:       err,
	}
	return stream.NewMessage("strategy", "execution", "status", entity)
}

type ExecutionRunCommand struct {
	Strategy          string   `json:"strategy"`
	ExecutionID       string   `json:"execution_id"`
	WorkerInstanceIDs []string `json:"worker_instance_ids"`

	Timestamp time.Time `json:"timestamp"`
	Symbols   []string  `json:"symbols"`
}

func NewExecutionRunCommand(strategy, executionID string, workerInstanceIDs []string, timestamp time.Time, symbols []string) (stream.Message, error) {
	entity := &ExecutionRunCommand{
		Strategy:          strategy,
		ExecutionID:       executionID,
		WorkerInstanceIDs: workerInstanceIDs,
		Timestamp:         timestamp,
		Symbols:           symbols,
	}
	return stream.NewMessage("strategy", "execution", "run", entity)
}

func RunStrategyExecutionOrchestration(strategy *entity.Strategy, execution *entity.Execution) (*stream.MessageOrchestration, error) {
	orchestration := stream.NewMessageOrchestration("run strategy execution")

	end := time.Now()
	start := end.Add(-time.Hour * 24 * time.Duration(strategy.MinDays))
	var serviceInstanceID string

	ingestMsg, err := financeStream.NewIngestCommand(strategy.Symbols, start, end)
	if err != nil {
		return nil, err
	}

	serviceInstanceID = serviceStream.NewInstanceID()
	startServiceMsg, err := serviceStream.NewServiceStartCommand(execution.Service, serviceInstanceID)
	if err != nil {
		return nil, err
	}
	startedMsg, err := NewUpdateExecutionStatusCommand(execution.ID, entity.ExecutionStatusStarted, nil)
	if err != nil {
		return nil, err
	}
	orchestration.AddStep("setup", []stream.Message{startServiceMsg, ingestMsg, startedMsg})

	msg, err := serviceStream.NewInstanceSanityCheckCommand([]string{serviceInstanceID})
	if err != nil {
		return nil, err
	}
	orchestration.AddStep("sanity check", []stream.Message{msg})

	runMsg, err := NewExecutionRunCommand(strategy.Name, execution.ID, []string{serviceInstanceID}, end, strategy.Symbols)
	if err != nil {
		return nil, err
	}
	runningMsg, err := NewUpdateExecutionStatusCommand(execution.ID, entity.ExecutionStatusRunning, nil)
	if err != nil {
		return nil, err
	}
	orchestration.AddStep("run", []stream.Message{runMsg, runningMsg})

	stopMsg, err := serviceStream.NewInstanceStopCommand(serviceInstanceID)
	if err != nil {
		return nil, err
	}
	completedMsg, err := NewUpdateExecutionStatusCommand(execution.ID, entity.ExecutionStatusCompleted, nil)
	if err != nil {
		return nil, err
	}

	orchestration.AddStep("teardown", []stream.Message{stopMsg, completedMsg})
	errMsg, err := NewUpdateExecutionStatusCommand(execution.ID, entity.ExecutionStatusFailed, nil)
	if err != nil {
		return nil, err
	}
	orchestration.SettFallback([]stream.Message{stopMsg, errMsg})
	return orchestration, nil
}
