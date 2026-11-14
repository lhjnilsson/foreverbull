package command

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

const blockedRetrySentence = "Stooq blocked the download for AAPL (it returned a bot-check page instead of data). Retry with --provider yahoo."

func TestBlockedBacktestMessageWhenTheBacktestIsRemoved(t *testing.T) {
	require.Equal(t, blockedRetrySentence, blockedBacktestMessage("test-backtest", errors.New(blockedRetrySentence), true))
}

func TestBlockedBacktestMessageWhenTheBacktestStays(t *testing.T) {
	require.Equal(t,
		"The backtest was partly saved and couldn't be removed. Run `fbull backtest delete test-backtest` first. "+
			blockedRetrySentence,
		blockedBacktestMessage("test-backtest", errors.New(blockedRetrySentence), false))
}

func TestCanceledIngestMessageWhenTheBacktestStays(t *testing.T) {
	require.Equal(t,
		"Couldn't load data for backtest 'oef-2024q1': the create was canceled. "+
			"The backtest was partly saved and couldn't be removed. Run `fbull backtest delete oef-2024q1` first. "+
			"Run `fbull backtest create` again.",
		endedIngestMessage("oef-2024q1", context.Canceled))
}

func TestTimedOutIngestMessageWhenTheBacktestStays(t *testing.T) {
	require.Equal(t,
		"Couldn't load data for backtest 'oef-2024q1': the create timed out. "+
			"The backtest was partly saved and couldn't be removed. Run `fbull backtest delete oef-2024q1` first. "+
			"Run `fbull backtest create` again.",
		endedIngestMessage("oef-2024q1", context.DeadlineExceeded))
}

func TestCanceledIngestPricesLeft(t *testing.T) {
	require.Equal(t,
		"Couldn't load data for backtest 'oef-2024q1': the create was canceled. "+
			"The backtest wasn't saved, but some of its prices couldn't be removed. They don't stop a new create. "+
			"Run `fbull backtest create` again.",
		endedIngestPricesLeft("oef-2024q1", context.Canceled))
}

func TestTimedOutIngestPricesLeft(t *testing.T) {
	require.Equal(t,
		"Couldn't load data for backtest 'oef-2024q1': the create timed out. "+
			"The backtest wasn't saved, but some of its prices couldn't be removed. They don't stop a new create. "+
			"Run `fbull backtest create` again.",
		endedIngestPricesLeft("oef-2024q1", context.DeadlineExceeded))
}
