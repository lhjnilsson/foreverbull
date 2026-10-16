package test_helper //nolint:stylecheck,revive

import (
	"context"
	"os"
	"strings"
	"testing"

	dockerNetwork "github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/stretchr/testify/require"

	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/testcontainers/testcontainers-go"

	"golang.org/x/sync/errgroup"
)

// NetworkID is the Docker network used when SetupEnvironment starts containers.
const (
	NetworkID = "foreverbull-testing-network"
)

// Containers selects which reused Testcontainers services to start.
// A nil or zero-value Containers starts no containers, does not create a Docker
// network, and does not disable Ryuk.
type Containers struct {
	Postgres bool
	NATS     bool
	Minio    bool
	Loki     bool
	Zipline  bool
}

func (c *Containers) requested() bool {
	if c == nil {
		return false
	}

	if c.Postgres || c.NATS || c.Minio || c.Zipline {
		return true
	}

	if !c.Loki {
		return false
	}

	_, disableLoki := os.LookupEnv("DISABLE_LOKI_LOGGING")

	return !disableLoki
}

func getOrCreateNetwork() error {
	c, err := client.NewClientWithOpts(
		client.FromEnv,
		client.WithAPIVersionNegotiation(),
	)
	if err != nil {
		return err
	}

	_, err = c.NetworkInspect(context.TODO(), NetworkID, dockerNetwork.InspectOptions{})
	if err != nil && strings.Contains(err.Error(), "not found") {
		nc := dockerNetwork.CreateOptions{
			Driver: "bridge",
			Labels: testcontainers.GenericLabels(),
		}
		_, err := c.NetworkCreate(context.TODO(), NetworkID, nc)
		if err != nil {
			if strings.Contains(err.Error(), "already exists") {
				return nil
			}
			return err
		}
	}
	return nil
}

// SetupEnv fills test env defaults (including test-only gRPC/backtest ports).
// It does not create a Docker network, disable Ryuk, or require a Docker daemon.
func SetupEnv(t *testing.T) {
	t.Helper()

	environment.SetupForTest(t)
	t.Setenv(environment.BacktestPortRangeStart, "37000")
	t.Setenv(environment.BacktestPortRangeEnd, "37100")
	t.Setenv(environment.GRPCPort, "60066")
}

func startContainers(t *testing.T, containers *Containers) {
	t.Helper()

	t.Setenv("TESTCONTAINERS_RYUK_DISABLED", "true")
	err := getOrCreateNetwork()
	require.NoError(t, err, "fail to create network")

	t.Setenv(environment.ServerAddress, "host.docker.internal")

	ctx := context.TODO()
	group, _ := errgroup.WithContext(ctx)

	var postgresURL, natsURL, minioURL, minioAccessKey, minioSecretKey string

	if containers.Postgres {
		group.Go(func() error {
			postgresURL = PostgresContainer(t, NetworkID)
			return nil
		})
	}

	if containers.NATS {
		group.Go(func() error {
			natsURL = NATSContainer(t, NetworkID)
			return nil
		})
	}

	if containers.Minio {
		group.Go(func() error {
			minioURL, minioAccessKey, minioSecretKey = MinioContainer(t, NetworkID)
			return nil
		})
	}

	_, disableLoki := os.LookupEnv("DISABLE_LOKI_LOGGING")
	if containers.Loki && !disableLoki {
		group.Go(func() error {
			LokiContainerAndLogging(t, NetworkID)
			return nil
		})
	}

	err = group.Wait()
	require.NoError(t, err, "fail to create environment")

	if postgresURL != "" {
		t.Setenv(environment.PostgresURL, postgresURL)
	}

	if natsURL != "" {
		t.Setenv(environment.NatsURL, natsURL)
		t.Setenv(environment.NatsDeliveryPolicy, "all")
	}

	if minioURL != "" {
		t.Setenv(environment.MinioURL, minioURL)
		t.Setenv(environment.MinioAccessKey, minioAccessKey)
		t.Setenv(environment.MinioSecretKey, minioSecretKey)
	}

	// Zipline depends on Postgres to be running
	if containers.Zipline {
		t.Setenv(environment.ZiplineContainerName, ZiplineContainer(t, NetworkID))
	}
}

// SetupEnvironment fills test env vars via SetupEnv. When containers is nil or
// all flags are false, that is all it does. When a flag is true, the matching
// reused named container is started on NetworkID. Postgres always gets a unique
// database per test.
func SetupEnvironment(t *testing.T, containers *Containers) {
	t.Helper()

	SetupEnv(t)

	if !containers.requested() {
		return
	}

	startContainers(t, containers)
}
