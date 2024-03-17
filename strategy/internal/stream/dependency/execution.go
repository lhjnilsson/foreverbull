package dependency

import (
	"context"
	"fmt"

	"github.com/lhjnilsson/foreverbull/internal/stream"
	serviceAPI "github.com/lhjnilsson/foreverbull/service/api"
	service "github.com/lhjnilsson/foreverbull/service/entity"
	"github.com/lhjnilsson/foreverbull/service/worker"
	ss "github.com/lhjnilsson/foreverbull/strategy/stream"
)

const ExecutionRunner stream.Dependency = "get_execution_runner"

type Execution interface {
	Configure(ctx context.Context, cfg *worker.Configuration) error
	Run(ctx context.Context, executionID string) error
}

type execution struct {
	worker worker.Pool
}

func (e *execution) Configure(ctx context.Context, cfg *worker.Configuration) error {
	return e.worker.ConfigureExecution(ctx, cfg)
}

func (e *execution) Run(ctx context.Context, executionID string) error {
	return nil
}

func GetExecution(ctx context.Context, message stream.Message) (interface{}, error) {
	return message.MustGet(ExecutionRunner).(Execution), nil
}

const WorkerPool stream.Dependency = "get_worker_pool"

func GetWorkerPool(ctx context.Context, message stream.Message) (interface{}, error) {
	command := ss.ExecutionRunCommand{}
	err := message.ParsePayload(&command)
	if err != nil {
		return nil, fmt.Errorf("error unmarshalling ExecutionRun payload: %w", err)
	}

	instances := make([]*service.Instance, len(command.WorkerInstanceIDs))
	http := message.MustGet("").(serviceAPI.Client)
	for _, workerInstanceID := range command.WorkerInstanceIDs {
		i, err := http.GetInstance(ctx, workerInstanceID)
		if err != nil {
			return nil, fmt.Errorf("error getting worker instance: %w", err)
		}
		instance := service.Instance{
			ID:    i.ID,
			Image: i.Image,
			Host:  i.Host,
			Port:  i.Port,
		}
		instances = append(instances, &instance)
	}

	pool, err := worker.NewPool(ctx, instances...)
	if err != nil {
		return nil, fmt.Errorf("error creating worker pool: %w", err)
	}
	return pool, nil
}
