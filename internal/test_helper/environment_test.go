package test_helper //nolint:stylecheck,revive

import (
	"context"
	"net/url"
	"os"
	"testing"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/stretchr/testify/require"
)

func TestSetupEnvironmentEnvOnlyDoesNotTouchDocker(t *testing.T) {
	cases := []struct {
		name       string
		containers *Containers
	}{
		{name: "nil", containers: nil},
		{name: "empty", containers: &Containers{}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			prevRyuk, hadRyuk := os.LookupEnv("TESTCONTAINERS_RYUK_DISABLED")
			t.Cleanup(func() {
				if hadRyuk {
					require.NoError(t, os.Setenv("TESTCONTAINERS_RYUK_DISABLED", prevRyuk))
				} else {
					require.NoError(t, os.Unsetenv("TESTCONTAINERS_RYUK_DISABLED"))
				}
			})
			require.NoError(t, os.Unsetenv("TESTCONTAINERS_RYUK_DISABLED"))

			t.Setenv(environment.DockerNetwork, "")
			t.Setenv(environment.ServerAddress, "")
			t.Setenv(environment.GRPCPort, "")
			t.Setenv(environment.BacktestPortRangeStart, "")
			t.Setenv(environment.BacktestPortRangeEnd, "")
			t.Setenv(environment.PostgresURL, "")

			SetupEnvironment(t, tc.containers)

			_, ryukSet := os.LookupEnv("TESTCONTAINERS_RYUK_DISABLED")
			require.False(t, ryukSet)
			require.Equal(t, environment.DockerNetworkDefault, environment.GetDockerNetworkName())
			require.Equal(t, environment.ServerAddressDefault, environment.GetServerAddress())
			require.Equal(t, "60066", environment.GetGRPCPort())
			require.Equal(t, 37000, environment.GetBacktestPortRangeStart())
			require.Equal(t, 37100, environment.GetBacktestPortRangeEnd())
			require.Equal(t, environment.PostgresURLDefault, environment.GetPostgresURL())
		})
	}
}

func TestSetupEnvFillsDefaultsWithoutContainers(t *testing.T) {
	t.Setenv(environment.GRPCPort, "")
	t.Setenv(environment.BacktestPortRangeStart, "")
	t.Setenv(environment.BacktestPortRangeEnd, "")

	SetupEnv(t)

	require.Equal(t, "60066", environment.GetGRPCPort())
	require.Equal(t, 37000, environment.GetBacktestPortRangeStart())
	require.Equal(t, 37100, environment.GetBacktestPortRangeEnd())
}

func TestSetupEnvironmentPostgresUniqueDatabasesAndReuse(t *testing.T) {
	SetupEnvironment(t, &Containers{Postgres: true})

	url1 := environment.GetPostgresURL()
	require.NotEqual(t, environment.PostgresURLDefault, url1)
	require.Equal(t, "true", os.Getenv("TESTCONTAINERS_RYUK_DISABLED"))
	require.Equal(t, NetworkID, environment.GetDockerNetworkName())
	require.Equal(t, "host.docker.internal", environment.GetServerAddress())
	require.Equal(t, 1, namedContainerCount(t, "foreverbull-testing-postgres"))
	pingPostgres(t, url1)

	var url2 string
	t.Run("second", func(t *testing.T) {
		SetupEnvironment(t, &Containers{Postgres: true})
		url2 = environment.GetPostgresURL()
		require.NotEqual(t, url1, url2)
		require.Equal(t, 1, namedContainerCount(t, "foreverbull-testing-postgres"))
		pingPostgres(t, url2)
	})

	parsed1, err := url.Parse(url1)
	require.NoError(t, err)
	parsed2, err := url.Parse(url2)
	require.NoError(t, err)
	require.Equal(t, parsed1.Host, parsed2.Host)
	require.NotEqual(t, parsed1.Path, parsed2.Path)
	pingPostgres(t, url1)
}

func pingPostgres(t *testing.T, connString string) {
	t.Helper()

	pool, err := pgxpool.New(context.Background(), connString)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	require.NoError(t, pool.Ping(context.Background()))
}

func namedContainerCount(t *testing.T, name string) int {
	t.Helper()

	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = cli.Close()
	})

	list, err := cli.ContainerList(context.Background(), container.ListOptions{
		All:     true,
		Filters: filters.NewArgs(filters.Arg("name", name)),
	})
	require.NoError(t, err)

	return len(list)
}
