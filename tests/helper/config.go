package helper

import (
	"context"
	"testing"

	"github.com/lhjnilsson/foreverbull/internal/config"
	"github.com/testcontainers/testcontainers-go/network"
)

type Containers struct {
	Postgres bool
	NATS     bool
	Minio    bool
}

func TestingConfig(t *testing.T, containers *Containers) *config.Config {
	t.Helper()
	if containers == nil {
		containers = &Containers{}
	}

	config, err := config.GetConfig()
	if err != nil {
		t.Fatal(err)
	}

	if containers.Postgres || containers.NATS || containers.Minio {
		network, err := network.New(context.TODO())
		if err != nil {
			t.Fatal(err)
		}
		config.Docker.Network = network.Name
		t.Cleanup(func() {
			if err := network.Remove(context.Background()); err != nil {
				t.Fatal(err)
			}
		})
	}

	if containers.Postgres {
		config.PostgresURI = PostgresContainer(t, config.Docker.Network)
	}
	if containers.NATS {
		config.NATSURI = NATSContainer(t, config.Docker.Network)
		config.NATS_DELIVERY_POLICY = "all"
	}
	if containers.Minio {
		config.MinioURI, config.MinioAccessKey, config.MinioSecretKey = MinioContainer(t, config.Docker.Network)
	}

	config.Hostname = "host.docker.internal"
	return config
}
