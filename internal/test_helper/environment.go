package test_helper

import (
	"context"
	"os"
	"testing"

	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/testcontainers/testcontainers-go/network"
	"golang.org/x/sync/errgroup"
)

type Containers struct {
	Postgres bool
	NATS     bool
	Minio    bool
	Loki     bool
}

func SetupEnvironment(t *testing.T, containers *Containers) {
	t.Helper()

	_ = environment.Setup()

	if containers == nil {
		containers = &Containers{}
	}

	ctx := context.TODO()
	group, _ := errgroup.WithContext(ctx)

	if containers.Postgres || containers.NATS || containers.Minio {
		network, err := network.New(context.TODO())
		if err != nil {
			t.Fatal(err)
		}

		os.Setenv(environment.DockerNetwork, network.Name)

		t.Cleanup(func() {
			if err := network.Remove(context.TODO()); err != nil {
				t.Fatal(err)
			}
		})
	}

	if containers.Postgres {
		group.Go(func() error {
			os.Setenv(environment.PostgresUrl, PostgresContainer(t, environment.GetDockerNetworkName()))
			return nil
		})
	}

	if containers.NATS {
		group.Go(func() error {
			os.Setenv(environment.NatsUrl, NATSContainer(t, environment.GetDockerNetworkName()))
			os.Setenv(environment.NatsDeliveryPolicy, "all")

			return nil
		})
	}

	if containers.Minio {
		group.Go(func() error {
			uri, accessKey, secretKey := MinioContainer(t, environment.GetDockerNetworkName())
			os.Setenv(environment.MinioUrl, uri)
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

	err := group.Wait()
	if err != nil {
		t.Fatal(err)
	}

	t.Setenv(environment.ServerAddress, "host.docker.internal")
}
