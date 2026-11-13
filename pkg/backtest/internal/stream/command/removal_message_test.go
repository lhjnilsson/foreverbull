package command

import (
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
		canceledIngestMessage("oef-2024q1"))
}
