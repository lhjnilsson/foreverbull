package dependency

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/lhjnilsson/foreverbull/internal/container"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/internal/stream"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/backtest"
)

const GetEngineKey stream.Dependency = "get_engine"

const (
	NumberOfTries = 30
	WaitTime      = time.Second / 3
)

func GetEngine(ctx context.Context, msg stream.Message) (interface{}, error) {
	containerEngine, isEngine := msg.MustGet(stream.ContainerEngineDep).(container.Engine)
	if !isEngine {
		return nil, errors.New("error casting container engine")
	}

	cont, err := containerEngine.Start(ctx, environment.GetBacktestImage(), "")
	if err != nil {
		return nil, fmt.Errorf("error starting container: %w", err)
	}

	for range NumberOfTries {
		health, err := cont.GetHealth()
		if err != nil {
			return nil, fmt.Errorf("error getting container health: %w", err)
		}

		if health == types.Healthy {
			break
		} else if health == types.Unhealthy {
			return nil, errors.New("container is unhealthy")
		}

		time.Sleep(WaitTime)
	}

	engine, err := backtest.NewZiplineEngine(ctx, cont, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating zipline engine: %w", err)
	}

	return engine, nil
}
