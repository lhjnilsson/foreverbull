package service

import (
	"context"

	finance "github.com/lhjnilsson/foreverbull/finance/entity"
	"github.com/lhjnilsson/foreverbull/service/entity"
	"github.com/lhjnilsson/foreverbull/service/internal/worker"
	"github.com/lhjnilsson/foreverbull/service/socket"
)

type WorkerPool interface {
	Configure(ctx context.Context, algorithm *entity.ServiceAlgorithm, databaseURL string) error
	Run(ctx context.Context) error
	Process(ctx context.Context, portfolio *finance.Portfolio, symbols []string) error
	Stop(ctx context.Context) error
}

func NewWorkerPool() (WorkerPool, *socket.Socket, error) {
	return worker.NewWorkerPool()
}
