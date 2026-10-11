package stream_test

import (
	"testing"

	"github.com/lhjnilsson/foreverbull/pkg/strategy/internal/stream"
	"github.com/stretchr/testify/require"
)

func TestNewExperimentRunCommand(t *testing.T) {
	t.Parallel()

	msg, err := stream.NewExperimentRunCommand("exp-1")
	require.NoError(t, err)
	require.Equal(t, "strategy", msg.GetMsg().Module)
	require.Equal(t, "experiment", msg.GetMsg().Component)
	require.Equal(t, "run", msg.GetMsg().Method)

	var payload stream.ExperimentRunCommand
	require.NoError(t, msg.ParsePayload(&payload))
	require.Equal(t, "exp-1", payload.ExperimentID)
}
