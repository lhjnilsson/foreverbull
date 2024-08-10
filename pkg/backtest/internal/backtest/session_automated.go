package backtest

import (
	"context"
	"fmt"

	"github.com/lhjnilsson/foreverbull/pkg/backtest/engine"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/entity"
	"github.com/lhjnilsson/foreverbull/pkg/service/worker"
	"github.com/rs/zerolog/log"
)

type automatedSession struct {
	session session `json:"-"`

	backtest engine.Engine `json:"-"`
	workers  worker.Pool   `json:"-"`

	executions *[]entity.Execution `json:"-"`
}

func (as *automatedSession) Run(chan<- bool, <-chan bool) error {
	for _, execution := range *as.executions {
		exec := NewExecution(as.backtest, as.workers)
		err := exec.Configure(context.Background(), &execution)
		if err != nil {
			return fmt.Errorf("failed to configure execution: %w", err)
		}

		err = as.session.executions.UpdateSimulationDetails(context.Background(), &execution)
		if err != nil {
			return fmt.Errorf("failed to update execution simulation details: %w", err)
		}

		err = exec.Run(context.TODO(), &execution)
		if err != nil {
			log.Error().Err(err).Msg("failed to run execution")
			return fmt.Errorf("failed to run execution: %w", err)
		}

		periods, err := exec.StoreDataFrameAndGetPeriods(context.Background(), execution.ID)
		if err != nil {
			log.Error().Err(err).Msg("failed to store data frame and get periods")
			return fmt.Errorf("failed to store data frame and get periods: %w", err)
		}
		for _, period := range *periods {
			err = as.session.periods.Store(context.Background(), execution.ID, &period)
			if err != nil {
				log.Error().Err(err).Msg("failed to store period")
				return fmt.Errorf("failed to store period: %w", err)
			}
		}
		log.Info().Str("execution_id", execution.ID).Msg("execution completed")
	}
	return nil
}

func (as *automatedSession) Stop(ctx context.Context) error {
	return as.workers.Close()
}
