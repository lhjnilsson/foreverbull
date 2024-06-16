package dependency

import (
	"context"
	"fmt"

	"github.com/lhjnilsson/foreverbull/internal/stream"
	finance "github.com/lhjnilsson/foreverbull/pkg/finance/entity"
	serviceAPI "github.com/lhjnilsson/foreverbull/pkg/service/api"
	service "github.com/lhjnilsson/foreverbull/pkg/service/entity"
	"github.com/lhjnilsson/foreverbull/pkg/service/worker"
	ss "github.com/lhjnilsson/foreverbull/pkg/strategy/stream"
)

const Trading stream.Dependency = "get_trading"

const ExecutionRunner stream.Dependency = "get_execution_runner"

type Execution interface {
	Configure(ctx context.Context) error
	Run(ctx context.Context, p *finance.Portfolio) (*[]finance.Order, error)
	Stop(ctx context.Context) error
}

type execution struct {
	worker  worker.Pool
	command ss.ExecutionRunCommand
}

func (e *execution) Configure(ctx context.Context) error {
	return nil
}

func (e *execution) Run(ctx context.Context, portfolio *finance.Portfolio) (*[]finance.Order, error) {
	orders, err := e.worker.Process(ctx, e.command.Timestamp, e.command.Symbols, portfolio)
	if err != nil {
		return nil, fmt.Errorf("error processing symbols: %w", err)
	}
	return orders, nil
}

func (e *execution) Stop(ctx context.Context) error {
	return nil
}

func GetExecution(ctx context.Context, message stream.Message) (interface{}, error) {
	command := ss.ExecutionRunCommand{}
	err := message.ParsePayload(&command)
	if err != nil {
		return nil, fmt.Errorf("error unmarshalling ExecutionRun payload: %w", err)
	}
	pool, err := message.Call(ctx, WorkerPool)
	if err != nil {
		return nil, fmt.Errorf("error getting worker pool: %w", err)
	}
	p := pool.(worker.Pool)
	return &execution{worker: p, command: command}, nil
}

const WorkerPool stream.Dependency = "get_worker_pool"
const ServiceAPI stream.Dependency = "get_service_api"

func GetWorkerPool(ctx context.Context, message stream.Message) (interface{}, error) {
	command := ss.ExecutionRunCommand{}
	err := message.ParsePayload(&command)
	if err != nil {
		return nil, fmt.Errorf("error unmarshalling ExecutionRun payload: %w", err)
	}

	instances := make([]*service.Instance, 0)
	http := message.MustGet(ServiceAPI).(serviceAPI.Client)
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
	pool, err := worker.NewPool(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating worker pool: %w", err)
	}
	return pool, nil
}
