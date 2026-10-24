package stream

import (
	"testing"

	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/require"
)

func TestNATSOptionsStayEmptyWithoutAUser(t *testing.T) {
	t.Setenv(environment.NatsUser, "")
	t.Setenv(environment.NatsPassword, "secret-value")
	require.Empty(t, NATSConnOptions())
}

func TestNATSOptionsUseTheBrokerUser(t *testing.T) {
	t.Setenv(environment.NatsUser, "broker")
	t.Setenv(environment.NatsPassword, "secret-value")
	applied := nats.GetDefaultOptions()
	for _, opt := range NATSConnOptions() {
		require.NoError(t, opt(&applied))
	}
	require.Equal(t, "broker", applied.User)
	require.Equal(t, "secret-value", applied.Password)
}
