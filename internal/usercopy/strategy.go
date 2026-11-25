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

// DeploymentListingNotStored is a deployment whose listing decision could
// not be read or saved. The deployment is not started.
func DeploymentListingNotStored(strategy string) string {
	name := strings.TrimSpace(strategy)
	const cause = "foreverbull couldn't save or read where its symbols are listed. The deployment wasn't started. " +
		"Run `fbenv status` to check that the environment is running, then run `fbull strategy deploy"
	if name == "" {
		return "Couldn't start the deployment: " + cause + "` again with the same options."
	}
	return "Couldn't start deployment '" + name + "': " + cause + " " + name + "` again with the same options."
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

// DryRunRecorded is the broker log line when a dry run read the portfolio of
// the deployment's trading profile. provider is that profile name, the value
// stored as backend, such as alpaca-paper.
func DryRunRecorded(provider string) string {
	return "Dry run: orders were worked out from the portfolio for trading profile '" + provider +
		"' and recorded only, and none were sent."
}

// DryRunEmptyPortfolio is the broker log line when a dry run has no trading
// profile, so the book is empty.
const DryRunEmptyPortfolio = "Dry run: foreverbull didn't read your trading account. Orders were worked out from an empty portfolio and recorded only, and none were sent."

// DryRunPortfolioUnreadable is the sentence stored on a dry-run deployment
// whose trading profile could not be read. strategy is the strategy name.
// provider is the profile name. An empty name is not quoted. The provider's
// own error is not included.
func DryRunPortfolioUnreadable(strategy, provider string) string {
	name := strings.TrimSpace(strategy)
	if name == "" {
		return fmt.Sprintf(
			"A dry-run deployment couldn't read the portfolio for trading profile '%s', so it didn't trade that session and stopped with an error. "+
				"Check that the account for '%s' is reachable and its keys are right, "+
				"then run `fbull strategy deploy` again with the same options.",
			provider, provider,
		)
	}
	return fmt.Sprintf(
		"Dry run for deployment '%s' couldn't read the portfolio for trading profile '%s', so it didn't trade that session and stopped with an error. "+
			"Check that the account for '%s' is reachable and its keys are right, "+
			"then run `fbull strategy deploy %s` again with the same options.",
		name, provider, provider, name,
	)
}

// UnknownTradingProfile is the deploy refusal when the name is not in the
// trading-profile registry.
func UnknownTradingProfile(name string) string {
	return fmt.Sprintf("There's no trading profile named '%s', so the deployment wasn't started. "+
		"Check the name against the trading profiles set up for foreverbull, then deploy again.", name)
}

// NoTradingProfiles is the deploy refusal when the broker has no registry,
// so a named profile cannot be used.
func NoTradingProfiles(name string) string {
	return fmt.Sprintf("This broker has no trading profiles set up, so it can't use '%s' and the deployment wasn't started. "+
		"Deploy without a trading profile to dry-run from an empty portfolio, or set up the profile, then deploy again.", name)
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
