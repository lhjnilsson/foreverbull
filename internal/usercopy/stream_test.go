package usercopy

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTaskStateNotRecorded(t *testing.T) {
	require.Equal(t, "foreverbull's task queue is full, so it couldn't record the task's progress", TaskStateNotRecorded)
}

func TestTaskFinishedNotSaved(t *testing.T) {
	require.Equal(t, "foreverbull couldn't record the task as finished. Run it again. If it repeats, please report it at https://github.com/lhjnilsson/foreverbull/issues with this message.", TaskFinishedNotSaved)
}

func TestTaskEndNotConfirmed(t *testing.T) {
	require.Equal(t, "foreverbull couldn't confirm whether the task finished. Don't assume it completed. Please report it at https://github.com/lhjnilsson/foreverbull/issues with this message.", TaskEndNotConfirmed)
	require.NotContains(t, TaskEndNotConfirmed, "Run it again")
}

func TestBacktestCreateQueueFull(t *testing.T) {
	require.Equal(t, "Couldn't load data for backtest 'spy': foreverbull's task queue is full, so it couldn't record the task's progress. "+
		"The backtest wasn't saved. Old tasks leave the queue after 24 hours, so run `fbull backtest create` again once they have. "+
		"Restarting fbenv won't clear the queue.", BacktestCreateQueueFull("spy", true))
	require.Equal(t, "Couldn't load data for backtest 'spy': foreverbull's task queue is full, so it couldn't record the task's progress. "+
		"The backtest was partly saved and couldn't be removed. Run `fbull backtest delete spy` first. "+
		"Old tasks leave the queue after 24 hours, so run `fbull backtest create` again once they have. "+
		"Restarting fbenv won't clear the queue.", BacktestCreateQueueFull("spy", false))
}

func TestBacktestIngestionMissing(t *testing.T) {
	require.Equal(t, "Couldn't start the session for backtest 'spy': its prices aren't loaded. "+
		"Delete it with `fbull backtest delete spy`, then create it again with `fbull backtest create`.",
		BacktestIngestionMissing("spy"))
}

func TestBacktestPricesLeft(t *testing.T) {
	require.Equal(t, "The backtest wasn't saved, but some of its prices couldn't be removed. They don't stop a new create.",
		BacktestPricesLeft())
}

func TestBacktestSessionQueueFull(t *testing.T) {
	require.Equal(t, "Couldn't start the session for backtest 'spy': foreverbull's task queue is full, so it couldn't record the task's progress. "+
		"Old tasks leave the queue after 24 hours, so run `fbull backtest run spy` again once they have. "+
		"Restarting fbenv won't clear the queue.", BacktestSessionQueueFull("spy"))
}

func TestExperimentQueueFull(t *testing.T) {
	require.Equal(t, "Couldn't start a backtest session for strategy experiment 'mean-rev': foreverbull's task queue is full, so it couldn't record the task's progress. "+
		"Old tasks leave the queue after 24 hours, so run the experiment again once they have. "+
		"Restarting fbenv won't clear the queue.", ExperimentQueueFull("mean-rev"))
}
