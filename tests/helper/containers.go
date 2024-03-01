package helper

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/nats"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func WaitTillContainersAreRemoved(t *testing.T, NetworkID string, timeout time.Duration) {
	t.Helper()
	ctx := context.TODO()
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	require.NoError(t, err)

	opts := container.ListOptions{
		Filters: filters.NewArgs(filters.Arg("network", NetworkID)),
	}
	opts.Filters.Add("label", "platform=foreverbull")
	opts.Filters.Add("label", "type=service")

	for {
		select {
		case <-ctx.Done():
			t.Error("timeout waiting for condition:", ctx.Err())
			return
		default:
			containers, err := cli.ContainerList(context.TODO(), opts)
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

const (
	PostgresImage = "postgres:alpine"
	NatsImage     = "nats:alpine"
	MinioImage    = "minio/minio:latest"
)

func PostgresContainer(t *testing.T, NetworkID string) (ConnectionString string) {
	t.Helper()

	// Disable logging, its very verbose otherwise
	// testcontainers.Logger = log.New(&ioutils.NopWriter{}, "", 0)

	dbName := strings.ToLower(strings.Replace(t.Name(), "/", "_", -1))
	container, err := postgres.RunContainer(context.TODO(),
		testcontainers.WithImage(PostgresImage),
		postgres.WithDatabase(dbName),
		testcontainers.WithEndpointSettingsModifier(func(settings map[string]*network.EndpointSettings) {
			settings[NetworkID] = &network.EndpointSettings{
				Aliases:   []string{"postgres"},
				NetworkID: NetworkID,
			}
		}),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).WithStartupTimeout(30*time.Second)),
	)
	require.NoError(t, err)

	ConnectionString, err = container.ConnectionString(context.TODO(), "sslmode=disable")
	require.NoError(t, err)

	t.Cleanup(func() {
		if err := container.Terminate(context.TODO()); err != nil {
			t.Fatal(err)
		}
	})
	return ConnectionString
}

func NATSContainer(t *testing.T, NetworkID string) (ConnectionString string) {
	t.Helper()

	container, err := nats.RunContainer(context.TODO(),
		testcontainers.WithImage(NatsImage),
		testcontainers.WithEndpointSettingsModifier(func(settings map[string]*network.EndpointSettings) {
			settings[NetworkID] = &network.EndpointSettings{
				Aliases:   []string{"nats"},
				NetworkID: NetworkID,
			}
		}),
	)
	require.NoError(t, err)

	ConnectionString, err = container.ConnectionString(context.TODO())
	require.NoError(t, err)

	t.Cleanup(func() {
		if err := container.Terminate(context.TODO()); err != nil {
			t.Fatal(err)
		}
	})
	return ConnectionString
}

func MinioContainer(t *testing.T, NetworkID string) (ConnectionString, AccessKey, SecretKey string) {
	t.Helper()

	container, err := RunContainer(context.TODO(),
		testcontainers.WithImage(MinioImage),
		WithUsername("minioadmin"),
		WithPassword("minioadmin"),
		testcontainers.WithEndpointSettingsModifier(func(settings map[string]*network.EndpointSettings) {
			settings[NetworkID] = &network.EndpointSettings{
				Aliases:   []string{"minio"},
				NetworkID: NetworkID,
			}
		}),
	)
	require.NoError(t, err)

	ConnectionString, err = container.ConnectionString(context.TODO())
	require.NoError(t, err)

	t.Cleanup(func() {
		if err := container.Terminate(context.TODO()); err != nil {
			t.Fatal(err)
		}
	})
	return ConnectionString, "minioadmin", "minioadmin"
}

type LoggingHook struct {
	LokiURL string
}

func (hook *LoggingHook) Run(e *zerolog.Event, level zerolog.Level, msg string) {
	payload := map[string]interface{}{
		"streams": []map[string]interface{}{
			{
				"stream": map[string]string{
					"job": "test",
				},
				"values": []interface{}{
					[]interface{}{fmt.Sprintf("%d", time.Now().UnixNano()), msg},
				},
			},
		},
	}
	marshalled, err := json.Marshal(payload)
	if err != nil {
		fmt.Println("Failed to marshal payload:", err)
		return
	}
	resp, err := http.Post(hook.LokiURL+"/loki/api/v1/push", "application/json", bytes.NewReader(marshalled))
	if err != nil {
		fmt.Println("Failed to send request:", err)
		return
	}
	fmt.Println("PUSHED LOGS TO LOKI: ", msg)
	if resp.StatusCode >= 300 {
		fmt.Println("Failed to send request:", resp)
	}
}

func LokiContainer(t *testing.T, NetworkID, dataFolder string) (ConnectionString string) {
	t.Helper()
	ctx := context.TODO()

	container := testcontainers.ContainerRequest{
		Image:        "grafana/loki:latest",
		ExposedPorts: []string{"3100/tcp"},
		WaitingFor:   wait.ForLog("will now accept requests"),
		HostConfigModifier: func(hostConfig *container.HostConfig) {
			hostConfig.Binds = []string{fmt.Sprintf("%s:/loki", dataFolder)}
		},
		Networks: []string{NetworkID},
	}

	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: container,
		Started:          true,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		c.Terminate(ctx)
	})
	host, err := c.Host(ctx)
	require.NoError(t, err)
	port, err := c.MappedPort(ctx, "3100/tcp")
	require.NoError(t, err)
	log.Logger = log.Hook(&LoggingHook{LokiURL: fmt.Sprintf("http://%s:%d", host, port.Int())})

	return fmt.Sprintf("http://%s:%d", host, port.Int())
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
