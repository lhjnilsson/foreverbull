package helper

import (
	"context"
	"fmt"
	"log"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/ioutils"
	"github.com/stretchr/testify/assert"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/nats"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func WaitTillContainersAreRemoved(t *testing.T, NetworkID string, timeout time.Duration) {
	t.Helper()
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		t.Error("Failed to create docker client:", err)
	}

	opts := container.ListOptions{
		Filters: filters.NewArgs(filters.Arg("network", NetworkID)),
	}
	opts.Filters.Add("label", "platform=foreverbull")
	opts.Filters.Add("label", "type=service")

	for {
		select {
		case <-ctx.Done():
			t.Error("timeout waiting for condition:", ctx.Err())
		default:
			containers, err := cli.ContainerList(context.Background(), opts)
			fmt.Println("Containers: ", containers)
			if err != nil {
				t.Error("Failed to list containers:", err)
			}
			if len(containers) == 0 {
				return
			}
			time.Sleep(time.Second / 4)
		}
	}
}

func PostgresContainer(t *testing.T, NetworkID string) (ConnectionString string) {
	t.Helper()

	// Disable logging, its very verbose otherwise
	testcontainers.Logger = log.New(&ioutils.NopWriter{}, "", 0)

	dbName := strings.ToLower(strings.Replace(t.Name(), "/", "_", -1))
	container, err := postgres.RunContainer(context.Background(),
		testcontainers.WithImage("postgres:alpine"),
		postgres.WithDatabase(dbName),
		testcontainers.WithEndpointSettingsModifier(func(settings map[string]*network.EndpointSettings) {
			settings[NetworkID] = &network.EndpointSettings{
				Aliases:   []string{"postgres"},
				NetworkID: NetworkID,
			}
		}),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).WithStartupTimeout(5*time.Second)),
	)
	assert.Nil(t, err)

	ConnectionString, err = container.ConnectionString(context.TODO(), "sslmode=disable")
	assert.Nil(t, err)

	t.Cleanup(func() {
		if err := container.Terminate(context.Background()); err != nil {
			t.Fatal(err)
		}
	})
	return ConnectionString
}

func NATSContainer(t *testing.T, NetworkID string) (ConnectionString string) {
	t.Helper()

	container, err := nats.RunContainer(context.Background(),
		testcontainers.WithImage("nats:alpine"),
		testcontainers.WithEndpointSettingsModifier(func(settings map[string]*network.EndpointSettings) {
			settings[NetworkID] = &network.EndpointSettings{
				Aliases:   []string{"nats"},
				NetworkID: NetworkID,
			}
		}),
	)
	assert.Nil(t, err)

	ConnectionString, err = container.ConnectionString(context.Background())
	assert.Nil(t, err)

	t.Cleanup(func() {
		if err := container.Terminate(context.Background()); err != nil {
			t.Fatal(err)
		}
	})
	return ConnectionString
}

func MinioContainer(t *testing.T, NetworkID string) (ConnectionString, AccessKey, SecretKey string) {
	t.Helper()

	container, err := RunContainer(context.Background(),
		testcontainers.WithImage("minio/minio:latest"),
		WithUsername("minioadmin"),
		WithPassword("minioadmin"),
		testcontainers.WithEndpointSettingsModifier(func(settings map[string]*network.EndpointSettings) {
			settings[NetworkID] = &network.EndpointSettings{
				Aliases:   []string{"minio"},
				NetworkID: NetworkID,
			}
		}),
	)
	assert.Nil(t, err)

	ConnectionString, err = container.ConnectionString(context.Background())
	assert.Nil(t, err)

	t.Cleanup(func() {
		if err := container.Terminate(context.Background()); err != nil {
			t.Fatal(err)
		}
	})
	return ConnectionString, "minioadmin", "minioadmin"
}

// Copied from https://github.com/testcontainers/testcontainers-go/blob/main/modules/minio/minio.go
// Remove below when new release is out with minio in upstream

const (
	defaultUser     = "minioadmin"
	defaultPassword = "minioadmin"
	defaultImage    = "docker.io/minio/minio:RELEASE.2024-01-16T16-07-38Z"
)

// MinioContainer represents the Minio container type used in the module
type minioContainer struct {
	testcontainers.Container
	Username string
	Password string
}

// WithUsername sets the initial username to be created when the container starts
// It is used in conjunction with WithPassword to set a user and its password.
// It will create the specified user. It must not be empty or undefined.
func WithUsername(username string) testcontainers.CustomizeRequestOption {
	return func(req *testcontainers.GenericContainerRequest) {
		req.Env["MINIO_ROOT_USER"] = username
	}
}

// WithPassword sets the initial password of the user to be created when the container starts
// It is required for you to use the Minio image. It must not be empty or undefined.
// This environment variable sets the root user password for Minio.
func WithPassword(password string) testcontainers.CustomizeRequestOption {
	return func(req *testcontainers.GenericContainerRequest) {
		req.Env["MINIO_ROOT_PASSWORD"] = password
	}
}

// ConnectionString returns the connection string for the minio container, using the default 9000 port, and
// obtaining the host and exposed port from the container.
func (c *minioContainer) ConnectionString(ctx context.Context) (string, error) {
	host, err := c.Host(ctx)
	if err != nil {
		return "", err
	}
	port, err := c.MappedPort(ctx, "9000/tcp")
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s:%s", host, port.Port()), nil
}

// RunContainer creates an instance of the Minio container type
func RunContainer(ctx context.Context, opts ...testcontainers.ContainerCustomizer) (*minioContainer, error) {
	req := testcontainers.ContainerRequest{
		Image:        defaultImage,
		ExposedPorts: []string{"9000/tcp"},
		WaitingFor:   wait.ForHTTP("/minio/health/live").WithPort("9000"),
		Env: map[string]string{
			"MINIO_ROOT_USER":     defaultUser,
			"MINIO_ROOT_PASSWORD": defaultPassword,
		},
		Cmd: []string{"server", "/data"},
	}

	genericContainerReq := testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	}

	for _, opt := range opts {
		opt.Customize(&genericContainerReq)
	}

	username := req.Env["MINIO_ROOT_USER"]
	password := req.Env["MINIO_ROOT_PASSWORD"]
	if username == "" || password == "" {
		return nil, fmt.Errorf("username or password has not been set")
	}

	container, err := testcontainers.GenericContainer(ctx, genericContainerReq)
	if err != nil {
		return nil, err
	}

	return &minioContainer{Container: container, Username: username, Password: password}, nil
}
