package stream_test

import (
	"testing"

	"github.com/lhjnilsson/foreverbull/pkg/finance/stream"
	"github.com/stretchr/testify/require"
)

func TestNewRunTradeOrchestration(t *testing.T) {
	t.Parallel()

	o, err := stream.NewRunTradeOrchestration("session-1", []string{"AAPL"})
	require.NoError(t, err)
	require.Len(t, o.Steps, 2)

	ingest := o.Steps[0].Messages[0]
	require.Equal(t, "finance", ingest.Module)
	require.Equal(t, "marketdata", ingest.Component)
	require.Equal(t, "ingest", ingest.Method)

	run := o.Steps[1].Messages[0]
	require.Equal(t, "finance", run.Module)
	require.Equal(t, "trading", run.Component)
	require.Equal(t, "run", run.Method)

	cmd, err := stream.NewRunTradeCommand("session-1", []string{"AAPL"})
	require.NoError(t, err)
	var payload stream.StartTradingSessionCommand
	require.NoError(t, cmd.ParsePayload(&payload))
	require.Equal(t, "session-1", payload.Id)
	require.Equal(t, []string{"AAPL"}, payload.Symbols)
}
