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
	g, _ := errgroup.WithContext(ctx)

	if containers.Postgres || containers.NATS || containers.Minio {
		network, err := network.New(context.TODO())
		if err != nil {
			t.Fatal(err)
		}
		os.Setenv(environment.DOCKER_NETWORK, network.Name)

		t.Cleanup(func() {
			if err := network.Remove(context.TODO()); err != nil {
				t.Fatal(err)
			}
		})
	}

	if containers.Postgres {
		g.Go(func() error {
			os.Setenv(environment.POSTGRES_URL, PostgresContainer(t, environment.GetDockerNetworkName()))
			return nil
		})
	}
	if containers.NATS {
		g.Go(func() error {
			os.Setenv(environment.NATS_URL, NATSContainer(t, environment.GetDockerNetworkName()))
			os.Setenv(environment.NATS_DELIVERY_POLICY, "all")
			return nil
		})
	}
	if containers.Minio {
		g.Go(func() error {
			uri, accessKey, secretKey := MinioContainer(t, environment.GetDockerNetworkName())
			os.Setenv(environment.MINIO_URL, uri)
			os.Setenv(environment.MINIO_ACCESS_KEY, accessKey)
			os.Setenv(environment.MINIO_SECRET_KEY, secretKey)
			return nil
		})
	}

	// If we run in Github CI, Loki will have issue creating folder and fail to start
	_, disableLoki := os.LookupEnv("DISABLE_LOKI_LOGGING")
	if containers.Loki && !disableLoki {
		g.Go(func() error {
			LokiContainerAndLogging(t, environment.GetDockerNetworkName())
			return nil
		})
	}
	err := g.Wait()
	if err != nil {
		t.Fatal(err)
	}
	os.Setenv(environment.SERVER_ADDRESS, "host.docker.internal")
}
