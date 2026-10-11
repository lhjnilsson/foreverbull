package command

import (
	"context"
	"fmt"
	"strings"

	"github.com/lhjnilsson/foreverbull/internal/container"
	"github.com/lhjnilsson/foreverbull/internal/postgres"
	"github.com/lhjnilsson/foreverbull/internal/stream"
	"github.com/lhjnilsson/foreverbull/pkg/backtest"
	"github.com/lhjnilsson/foreverbull/pkg/service/internal/repository"
	"github.com/lhjnilsson/foreverbull/pkg/service/internal/stream/dependency"
	ss "github.com/lhjnilsson/foreverbull/pkg/service/stream"
)

func UpdateServiceStatus(ctx context.Context, message stream.Message) error {
	return nil
}

func ServiceStart(ctx context.Context, message stream.Message) error {
	postgres, isDB := message.MustGet(stream.DBDep).(postgres.Query)
	if !isDB {
		return fmt.Errorf("db dependency casting failed")
	}

	runtime := message.MustGet(dependency.ContainerDep).(container.Engine)

	command := ss.ServiceStartCommand{}

	err := message.ParsePayload(&command)
	if err != nil {
		return fmt.Errorf("error unmarshalling ServiceStart payload: %w", err)
	}

	services := repository.Service{Conn: postgres}
	_, err = services.Get(ctx, command.Image)

	if err != nil {
		_, crErr := services.Create(ctx, command.Image)
		if crErr != nil {
			return fmt.Errorf("error creating service: %w", crErr)
		}
	}

	provider := strings.TrimSpace(command.OHLCProvider)
	if provider == "" && strings.TrimSpace(command.Backtest) != "" {
		stored, getErr := backtest.NewStore(postgres).GetBacktest(ctx, strings.TrimSpace(command.Backtest))
		if getErr != nil {
			return fmt.Errorf("error getting backtest %s: %w", command.Backtest, getErr)
		}
		provider = strings.TrimSpace(stored.GetProvider())
	}
	ctx = container.WithOHLCProvider(ctx, provider)

	/*extraLabels := map[string]string{
	"orchestration_id": message.GetOrchestrationID(),
	}*/
	_, err = runtime.Start(ctx, command.Image, command.InstanceID)
	if err != nil {
		return fmt.Errorf("error starting container: %w", err)
	}

	instances := repository.Instance{Conn: postgres}

	_, err = instances.Create(ctx, command.InstanceID, &command.Image)
	if err != nil {
		return fmt.Errorf("error creating instance: %w", err)
	}

	return nil
}
