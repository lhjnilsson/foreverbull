package command

import (
	"context"
	"fmt"

	"github.com/lhjnilsson/foreverbull/internal/postgres"
	"github.com/lhjnilsson/foreverbull/internal/stream"
	"github.com/lhjnilsson/foreverbull/service/container"
	"github.com/lhjnilsson/foreverbull/service/internal/repository"
	"github.com/lhjnilsson/foreverbull/service/internal/stream/dependency"
	ss "github.com/lhjnilsson/foreverbull/service/stream"
)

func UpdateServiceStatus(ctx context.Context, message stream.Message) error {
	db := message.MustGet(stream.DBDep).(postgres.Query)

	command := ss.UpdateServiceStatusCommand{}
	err := message.ParsePayload(&command)
	if err != nil {
		return fmt.Errorf("error unmarshalling UpdateServiceStatus payload: %w", err)
	}

	services := repository.Service{Conn: db}
	err = services.UpdateStatus(ctx, command.Name, command.Status, command.Error)
	if err != nil {
		return fmt.Errorf("error updating service status: %w", err)
	}
	return nil
}

func ServiceStart(ctx context.Context, message stream.Message) error {
	db := message.MustGet(stream.DBDep).(postgres.Query)
	container := message.MustGet(dependency.ContainerDep).(container.Container)

	command := ss.ServiceStartCommand{}
	err := message.ParsePayload(&command)
	if err != nil {
		return fmt.Errorf("error unmarshalling ServiceStart payload: %w", err)
	}

	services := repository.Service{Conn: db}
	service, err := services.Get(ctx, command.Name)
	if err != nil {
		return fmt.Errorf("error getting service: %w", err)
	}

	extraLabels := map[string]string{
		"orchestration_id": message.GetOrchestrationID(),
	}

	_, err = container.Start(ctx, service.Name, service.Image, command.InstanceID, extraLabels)
	if err != nil {
		return fmt.Errorf("error starting container: %w", err)
	}

	instances := repository.Instance{Conn: db}
	_, err = instances.Create(ctx, command.InstanceID, service.Name)
	if err != nil {
		return fmt.Errorf("error creating instance: %w", err)
	}
	return nil
}
