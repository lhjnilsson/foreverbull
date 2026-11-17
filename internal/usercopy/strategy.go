package usercopy

import (
	"fmt"
	"strings"
)

// Plain causes of a failed experiment run, in the order they happen.
const (
	CauseRunFailed          = "the backtest run failed"
	CauseExecutionNotStored = "the run couldn't be recorded"
	CauseFailureNotStored   = "the failure couldn't be saved"
	CauseNotReloaded        = "the saved run couldn't be read back"
	CauseNotSent            = "fbull couldn't be told which step failed"
	// CauseStartNotSent is a run that never started: fbull couldn't be
	// told it was running, so it isn't started.
	CauseStartNotSent = "fbull couldn't be told the run had started"
)

// ExperimentCommand is the command that runs an experiment of strategy on
// backtest.
func ExperimentCommand(strategy, backtest string) string {
	return fmt.Sprintf("fbull strategy experiment %s --backtest %s", strategy, backtest)
}

// ExperimentRunFailed is the sentence for an experiment that couldn't run.
// One or two causes fit the sentence ("A and B."); three or more go one per
// line under the lead, each ending with a period.
func ExperimentRunFailed(strategy, backtest string, causes []string) string {
	step := RetryStep(ExperimentCommand(strategy, backtest))
	if len(causes) >= 3 {
		return fmt.Sprintf("Couldn't run experiment '%s':\n  %s.\n%s", strategy, strings.Join(causes, ".\n  "), step)
	}
	return fmt.Sprintf("Couldn't run experiment '%s': %s. %s", strategy, strings.Join(causes, " and "), step)
}

// DeploymentStartFailed is the sentence for a deployment whose runner
// couldn't start. notStored is a failure that couldn't be saved either: the
// deployment still reads RUNNING and would resume on the next start, so the
// step stops it before deploying again.
func DeploymentStartFailed(strategy string, notStored bool) string {
	if notStored {
		return fmt.Sprintf("Couldn't start deployment '%s', and the failure couldn't be saved, so the deployment still shows as running. "+
			"Run `fbenv status` to check that the environment is running, "+
			"then run `fbull strategy stop %s`, then `fbull strategy deploy %s`.", strategy, strategy, strategy)
	}
	return fmt.Sprintf("Couldn't start deployment '%s'. %s", strategy, RetryStep("fbull strategy deploy "+strategy))
}

// MissedSessionClose is the sentence stored on a deployment that was down
// across one or more session closes when skip-missed is off. strategy is the
// strategy name. dates are the missed session dates, YYYY-MM-DD, oldest
// first. An empty name is not quoted. One date names that close. Two dates
// are joined with "and". Three or more name the first, the last, and the count.
// No dates is an error: the sentence is not rendered.
func MissedSessionClose(strategy string, dates []string) (string, error) {
	missed, err := missedCloses(dates)
	if err != nil {
		return "", err
	}
	name := strings.TrimSpace(strategy)
	who := "A deployment"
	command := "fbull strategy deploy"
	if name != "" {
		who = "Deployment '" + name + "'"
		command = "fbull strategy deploy " + name
	}
	return who + " " + missed +
		" Run `" + command + "` again with the same options plus --skip-missed to skip missed closes and keep trading.", nil
}

func missedCloses(dates []string) (string, error) {
	switch len(dates) {
	case 0:
		return "", fmt.Errorf("missed close has no session date")
	case 1:
		return "missed the close on " + dates[0] + ", so it didn't trade that session and stopped with an error.", nil
	case 2:
		return "missed the closes on " + dates[0] + " and " + dates[1] +
			", so it didn't trade those sessions and stopped with an error.", nil
	default:
		return fmt.Sprintf(
			"missed the closes on %s to %s (%d sessions), so it didn't trade those sessions and stopped with an error.",
			dates[0], dates[len(dates)-1], len(dates),
		), nil
	}
}

// DeploymentStopFailed is the sentence for a deployment whose runner
// couldn't stop.
func DeploymentStopFailed(strategy string) string {
	return fmt.Sprintf("Couldn't stop deployment '%s'. %s", strategy, RetryStep("fbull strategy stop "+strategy))
}

// ExecutionBacktestCheckFailed is the sentence when the check that the
// caller may read the backtest of an execution errored. A denied read is an
// answer, not this sentence. It surfaces only on the GetExperiment API,
// which no fbull command calls.
func ExecutionBacktestCheckFailed(executionID, backtest string) string {
	return fmt.Sprintf("Couldn't check your access to backtest '%s' for execution %s. %s", backtest, executionID, RetryStep(""))
}
