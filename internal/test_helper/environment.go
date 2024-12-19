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

const (
	NetworkID = "foreverbull-testing-network"
)

type Containers struct {
	Postgres bool
	NATS     bool
	Minio    bool
	Loki     bool
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

func SetupEnvironment(t *testing.T, containers *Containers) {
	t.Helper()

	_ = environment.Setup()

	if containers == nil {
		containers = &Containers{}
	}

	ctx := context.TODO()
	group, _ := errgroup.WithContext(ctx)

	os.Setenv("TESTCONTAINERS_RYUK_DISABLED", "true") // Disable ryuk that cleans up containers
	err := getOrCreateNetwork()
	if err != nil {
		require.NoError(t, err, "fail to create network")
	}

	os.Setenv(environment.DockerNetwork, NetworkID)

	if containers.Postgres {
		group.Go(func() error {
			os.Setenv(environment.PostgresURL, PostgresContainer(t, NetworkID))
			return nil
		})
	}

	if containers.NATS {
		group.Go(func() error {
			os.Setenv(environment.NatsURL, NATSContainer(t, NetworkID))
			os.Setenv(environment.NatsDeliveryPolicy, "all")

			return nil
		})
	}

	if containers.Minio {
		group.Go(func() error {
			uri, accessKey, secretKey := MinioContainer(t, NetworkID)
			os.Setenv(environment.MinioURL, uri)
			os.Setenv(environment.MinioAccessKey, accessKey)
			os.Setenv(environment.MinioSecretKey, secretKey)

			return nil
		})
	}

	// If we run in Github CI, Loki will have issue creating folder and fail to start
	_, disableLoki := os.LookupEnv("DISABLE_LOKI_LOGGING")
	if containers.Loki && !disableLoki {
		group.Go(func() error {
			LokiContainerAndLogging(t, environment.GetDockerNetworkName())
			return nil
		})
	}

	err = group.Wait()
	if err != nil {
		require.NoError(t, err, "fail to create environment")
	}

	t.Setenv(environment.ServerAddress, "host.docker.internal")
}
