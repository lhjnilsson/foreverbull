package stream_test

import (
	"testing"

	"github.com/lhjnilsson/foreverbull/pkg/finance/stream"
	"github.com/stretchr/testify/require"
)

func TestNewIngestCommand(t *testing.T) {
	t.Parallel()

	end := "2023-12-31"
	msg, err := stream.NewIngestCommand("XNYS", []string{"AAPL", "MSFT"}, "2022-01-01", &end)
	require.NoError(t, err)
	require.NotNil(t, msg)

	got := msg.GetMsg()
	require.Equal(t, "finance", got.Module)
	require.Equal(t, "marketdata", got.Component)
	require.Equal(t, "ingest", got.Method)
	require.NotEmpty(t, got.Id)

	var payload stream.IngestCommand
	require.NoError(t, msg.ParsePayload(&payload))
	require.Equal(t, "XNYS", payload.Calendar)
	require.Equal(t, []string{"AAPL", "MSFT"}, payload.Symbols)
	require.Equal(t, "2022-01-01", payload.Start)
	require.NotNil(t, payload.End)
	require.Equal(t, end, *payload.End)
}
