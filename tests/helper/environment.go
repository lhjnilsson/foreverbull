package helper

import (
	"context"
	"os"
	"testing"

	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/testcontainers/testcontainers-go/network"
)

type Containers struct {
	Postgres bool
	NATS     bool
	Minio    bool
}

func SetupEnvironment(t *testing.T, containers *Containers) {
	t.Helper()
	_ = environment.Setup()

	if containers == nil {
		containers = &Containers{}
	}

	if containers.Postgres || containers.NATS || containers.Minio {
		network, err := network.New(context.TODO())
		if err != nil {
			t.Fatal(err)
		}
		os.Setenv(environment.DOCKER_NETWORK, network.Name)
		t.Cleanup(func() {
			if err := network.Remove(context.Background()); err != nil {
				t.Fatal(err)
			}
		})
	}

	if containers.Postgres {
		os.Setenv(environment.POSTGRES_URL, PostgresContainer(t, environment.GetDockerNetworkName()))
	}
	if containers.NATS {
		os.Setenv(environment.NATS_URL, NATSContainer(t, environment.GetDockerNetworkName()))
		os.Setenv(environment.NATS_DELIVERY_POLICY, "all")
	}
	if containers.Minio {
		uri, accessKey, secretKey := MinioContainer(t, environment.GetDockerNetworkName())
		os.Setenv(environment.MINIO_URL, uri)
		os.Setenv(environment.MINIO_ACCESS_KEY, accessKey)
		os.Setenv(environment.MINIO_SECRET_KEY, secretKey)
	}

	os.Setenv(environment.SERVER_ADDRESS, "host.docker.internal")
}
