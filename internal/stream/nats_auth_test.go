package stream

import (
	"testing"

	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/stretchr/testify/require"
)

func TestNATSOptionsStayEmptyWithoutAUser(t *testing.T) {
	t.Setenv(environment.NatsUser, "")
	t.Setenv(environment.NatsPassword, "secret-value")
	require.Empty(t, natsConnOptions())
}

func TestNATSOptionsUseTheBrokerUser(t *testing.T) {
	t.Setenv(environment.NatsUser, "broker")
	t.Setenv(environment.NatsPassword, "secret-value")
	opts := natsConnOptions()
	require.Len(t, opts, 1)
}
