package usercopy

import "fmt"

// CloseOnUniverseDropFixed is the refusal when a create would change the
// option on a backtest that already exists. The setting is chosen once.
const CloseOnUniverseDropFixed = "Close on universe drop is set when the backtest is created and can't be changed. " +
	"To use a different setting, create a new backtest with `fbull backtest create`."

// BacktestNotSaved says a failed create kept nothing. It is only true when
// the backtest was removed.
const BacktestNotSaved = "The backtest wasn't saved."

// BacktestNotRemoved replaces BacktestNotSaved when the backtest couldn't be
// removed after a failed create.
const BacktestNotRemoved = "The backtest was partly saved and couldn't be removed."

// BacktestDeleteFirst is the next step when the backtest couldn't be
// removed: its own sentence to delete it first, then the step the failure
// gives when it was removed, exactly as that step reads.
func BacktestDeleteFirst(backtest, next string) string {
	return fmt.Sprintf("Run `fbull backtest delete %s` first. %s", backtest, next)
}

// BacktestSavedAndNext is the saved sentence and the next step of a failed
// create: BacktestNotSaved and next when the backtest was removed, otherwise
// BacktestNotRemoved and next behind the delete-first step.
func BacktestSavedAndNext(backtest, next string, removed bool) (string, string) {
	if removed {
		return BacktestNotSaved, next
	}
	return BacktestNotRemoved, BacktestDeleteFirst(backtest, next)
}

// BacktestCleanupInternalCause is the cause when a failed create's cleanup
// message has no not-removed sentence to rewrite. The user is asked to
// report it.
const BacktestCleanupInternalCause = "foreverbull hit an internal error while cleaning up."

// BacktestDeleteThenCreate is the one-sentence step when a partly saved
// backtest has to be deleted before create is run again.
func BacktestDeleteThenCreate(name string) string {
	return fmt.Sprintf("Run `fbull backtest delete %s` first, then run `fbull backtest create` again.", name)
}

// BacktestCleanupInternal is the sentence for that cleanup failure.
// pricesKept is the not-removed clause that also names the prices that stayed.
func BacktestCleanupInternal(name, pricesKept string) string {
	return fmt.Sprintf("Couldn't load data for backtest '%s': %s %s %s %s",
		name, BacktestCleanupInternalCause, pricesKept, BacktestDeleteThenCreate(name), ReportIt)
}
