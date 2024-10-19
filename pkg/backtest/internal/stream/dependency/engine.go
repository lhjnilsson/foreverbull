package dependency

import (
	"context"
	"fmt"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/lhjnilsson/foreverbull/internal/container"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/internal/stream"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/backtest"
)

const GetEngineKey stream.Dependency = "get_engine"

func GetEngine(ctx context.Context, msg stream.Message) (interface{}, error) {
	containerEngine := msg.MustGet(stream.ContainerEngineDep).(container.Engine)

	cont, err := containerEngine.Start(ctx, environment.GetBacktestImage(), "")
	if err != nil {
		return nil, fmt.Errorf("error starting container: %v", err)
	}

	for i := 0; i < 30; i++ {
		health, err := cont.GetHealth()
		if err != nil {
			return nil, fmt.Errorf("error getting container health: %v", err)
		}

		if health == types.Healthy {
			break
		} else if health == types.Unhealthy {
			return nil, fmt.Errorf("container is unhealthy")
		}

		time.Sleep(time.Second / 3)
	}

	ze, err := backtest.NewZiplineEngine(ctx, cont, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating zipline engine: %v", err)
	}

	return ze, nil
}
