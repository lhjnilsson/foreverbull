package usercopy

import "fmt"

// TaskStateNotRecorded is the plain cause when the foreverbull task stream
// is full and refused a task or one of its states. The task is failed
// rather than left waiting for a state that was never stored.
const TaskStateNotRecorded = "foreverbull's task queue is full, so it couldn't record the task's progress"

// TaskFinishedNotSaved is the plain cause when a task finished and the
// read showed that success was not stored. The sentence tells the user
// to run the task again.
const TaskFinishedNotSaved = "foreverbull couldn't record the task as finished. Run it again. If it repeats, please report it at https://github.com/lhjnilsson/foreverbull/issues with this message."

// TaskEndNotConfirmed is the plain cause when a final outcome could not
// be checked, including a follow-up read that failed. The sentence does
// not tell the user to run the task again.
const TaskEndNotConfirmed = "foreverbull couldn't confirm whether the task finished. Don't assume it completed. Please report it at https://github.com/lhjnilsson/foreverbull/issues with this message."

// queueFullNext is the step after a full task queue: wait for old tasks to
// leave it (the stream keeps a message for 24 hours), then retry. Restarting
// the environment keeps the stream's volume, so it doesn't help.
func queueFullNext(retry string) string {
	return fmt.Sprintf("Old tasks leave the queue after 24 hours, so %s once they have. Restarting fbenv won't clear the queue.", retry)
}

// BacktestCreateQueueFullNext is the next step for a backtest create that
// failed on a full task queue.
var BacktestCreateQueueFullNext = queueFullNext("run `fbull backtest create` again")

// BacktestCreateQueueFull is the sentence for a backtest create that failed
// on a full task queue. removed says whether the backtest the create stored
// was removed; if not, it was partly saved and is deleted first.
func BacktestCreateQueueFull(backtest string, removed bool) string {
	saved, next := BacktestSavedAndNext(backtest, BacktestCreateQueueFullNext, removed)
	return fmt.Sprintf("Couldn't load data for backtest '%s': %s. %s %s", backtest, TaskStateNotRecorded, saved, next)
}

// BacktestIngestionMissing is the sentence when a session starts and that
// backtest's prices are not loaded. Another backtest's ingestion is not used.
// Experiments and the Web execution start reach the same failure, so the
// next step ends at create.
func BacktestIngestionMissing(backtest string) string {
	return fmt.Sprintf("Couldn't start the session for backtest '%s': its prices aren't loaded. "+
		"Delete it with `fbull backtest delete %s`, then create it again with `fbull backtest create`.",
		backtest, backtest)
}

// BacktestPricesLeft replaces "The backtest wasn't saved." when stored prices
// could not be removed. It has no next step of its own; the message keeps
// the step it already had. Those prices do not refuse a new create. With
// auth off a session can still read them until that create replaces the name.
func BacktestPricesLeft() string {
	return "The backtest wasn't saved, but some of its prices couldn't be removed. They don't stop a new create."
}

// BacktestSessionQueueFull is the sentence for a backtest session that
// couldn't start on a full task queue.
func BacktestSessionQueueFull(backtest string) string {
	return fmt.Sprintf("Couldn't start the session for backtest '%s': %s. %s", backtest, TaskStateNotRecorded,
		queueFullNext(fmt.Sprintf("run `fbull backtest run %s` again", backtest)))
}

// ExperimentQueueFull is the sentence for a strategy experiment whose
// backtest session couldn't start on a full task queue.
func ExperimentQueueFull(strategy string) string {
	return fmt.Sprintf("Couldn't start a backtest session for strategy experiment '%s': %s. %s", strategy, TaskStateNotRecorded,
		queueFullNext("run the experiment again"))
}
