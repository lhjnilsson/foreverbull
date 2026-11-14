package stream_test

import (
	"encoding/json"
	"testing"

	"github.com/lhjnilsson/foreverbull/pkg/finance/stream"
	"github.com/stretchr/testify/require"
)

func TestNewRunTradeOrchestration(t *testing.T) {
	t.Parallel()

	o, err := stream.NewRunTradeOrchestration("session-1", []string{"AAPL"}, "stooq", "alpaca-paper")
	require.NoError(t, err)
	require.Len(t, o.Steps, 2)

	ingest := o.Steps[0].Messages[0]
	require.Equal(t, "finance", ingest.Module)
	require.Equal(t, "marketdata", ingest.Component)
	require.Equal(t, "ingest", ingest.Method)
	var ingestPayload stream.IngestCommand
	require.NoError(t, json.Unmarshal(ingest.GetPayload(), &ingestPayload))
	require.Equal(t, "stooq", ingestPayload.Provider)
	require.Equal(t, "alpaca-paper", ingestPayload.Profile)
	require.True(t, ingestPayload.Trading)

	run := o.Steps[1].Messages[0]
	require.Equal(t, "finance", run.Module)
	require.Equal(t, "trading", run.Component)
	require.Equal(t, "run", run.Method)

	cmd, err := stream.NewRunTradeCommand("session-1", "yahoo", "", []string{"AAPL"})
	require.NoError(t, err)
	var payload stream.StartTradingSessionCommand
	require.NoError(t, cmd.ParsePayload(&payload))
	require.Equal(t, "session-1", payload.Id)
	require.Equal(t, []string{"AAPL"}, payload.Symbols)
	require.Equal(t, "yahoo", payload.Provider)
}
