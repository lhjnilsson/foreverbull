package test_helper //nolint:stylecheck,revive

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	dockercontainer "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	mobycontainer "github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/network"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/minio"
	"github.com/testcontainers/testcontainers-go/modules/nats"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// WaitTillContainersAreRemoved waits until no Foreverbull service containers
// remain on networkID, or fails t after timeout.
func WaitTillContainersAreRemoved(t *testing.T, networkID string, timeout time.Duration) {
	t.Helper()

	ctx := context.TODO()

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	require.NoError(t, err)

	opts := dockercontainer.ListOptions{
		Filters: filters.NewArgs(filters.Arg("network", networkID)),
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

			time.Sleep(time.Second)
		}
	}
}

const (
	PostgresImage = "postgres:alpine"
	NatsImage     = "nats:alpine"
	MinioImage    = "pgsty/silo:RELEASE.2026-09-16T00-00-00Z"
)

// PostgresContainer starts or reuses the named Postgres test container on
// networkID and returns a connection string to a unique database for t.
func PostgresContainer(t *testing.T, networkID string) (ConnectionString string) {
	t.Helper()

	// Disable logging, its very verbose otherwise
	// testcontainers.Logger = log.New(&ioutils.NopWriter{}, "", 0)

	reuse := testcontainers.CustomizeRequestOption(
		func(req *testcontainers.GenericContainerRequest) error {
			req.Reuse = true
			req.Name = "foreverbull-testing-postgres"
			return nil
		},
	)

	container, err := postgres.Run(context.TODO(),
		PostgresImage,
		postgres.WithDatabase("to_be_substituted"),
		testcontainers.WithEndpointSettingsModifier(func(settings map[string]*network.EndpointSettings) {
			settings[networkID] = &network.EndpointSettings{
				Aliases:   []string{"postgres"},
				NetworkID: networkID,
			}
		}),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).WithStartupTimeout(time.Minute)),
		reuse,
	)
	require.NoError(t, err)

	connectionString, err := container.ConnectionString(context.TODO(), "sslmode=disable")
	require.NoError(t, err)

	name := t.Name() + strings.Split(uuid.NewString(), "-")[0]
	dbName := strings.ToLower(strings.ReplaceAll(name, "/", "_"))

	conn, err := pgxpool.New(context.TODO(), connectionString)
	require.NoError(t, err)
	_, err = conn.Exec(context.TODO(), fmt.Sprintf("DROP DATABASE IF EXISTS %s WITH (FORCE) ", dbName))
	require.NoError(t, err)

	_, err = conn.Exec(context.TODO(), fmt.Sprintf("CREATE DATABASE  %s", dbName))
	require.NoError(t, err)
	t.Cleanup(func() {
		_, err := conn.Exec(context.TODO(), fmt.Sprintf("DROP DATABASE IF EXISTS %s WITH (FORCE) ", dbName))
		require.NoError(t, err)
	})
	return strings.Replace(connectionString, "to_be_substituted", dbName, 1)
}

// NATSContainer starts or reuses the named NATS test container on networkID.
func NATSContainer(t *testing.T, networkID string) (ConnectionString string) {
	t.Helper()

	reuse := testcontainers.CustomizeRequestOption(
		func(req *testcontainers.GenericContainerRequest) error {
			req.Reuse = true
			req.Name = "foreverbull-testing-nats"
			return nil
		},
	)

	container, err := nats.Run(context.TODO(),
		NatsImage,
		testcontainers.WithEndpointSettingsModifier(func(settings map[string]*network.EndpointSettings) {
			settings[networkID] = &network.EndpointSettings{
				Aliases:   []string{"nats"},
				NetworkID: networkID,
			}
		}),
		reuse,
	)
	require.NoError(t, err, "Failed to start NATS container")

	attempts := 12
	timeout := time.Second / 4

	var connectionString string
	for range attempts {
		connectionString, err = container.ConnectionString(context.TODO())
		if err == nil {
			break
		}

		time.Sleep(timeout)

		continue
	}

	require.NoError(t, err, "Failed to get NATS connection string")
	return connectionString
}

// MinioContainer starts or reuses the named Minio test container on networkID.
func MinioContainer(t *testing.T, networkID string) (ConnectionString, AccessKey, SecretKey string) {
	t.Helper()

	reuse := testcontainers.CustomizeRequestOption(
		func(req *testcontainers.GenericContainerRequest) error {
			req.Reuse = true
			req.Name = "foreverbull-testing-minio"
			return nil
		},
	)

	container, err := minio.Run(context.TODO(),
		MinioImage,
		minio.WithUsername("minioadmin"),
		minio.WithPassword("minioadmin"),
		testcontainers.WithEndpointSettingsModifier(func(settings map[string]*network.EndpointSettings) {
			settings[networkID] = &network.EndpointSettings{
				Aliases:   []string{"minio"},
				NetworkID: networkID,
			}
		}),
		reuse,
	)
	require.NoError(t, err)

	connectionString, err := container.ConnectionString(context.TODO())
	require.NoError(t, err)

	return connectionString, "minioadmin", "minioadmin"
}

// LokiLogger buffers test log lines and publishes them to Loki on cleanup.
type LokiLogger struct {
	mu      sync.Mutex
	LokiURL string
	entries map[int64]string
}

func (l *LokiLogger) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.entries[time.Now().UnixNano()] = string(p)

	return len(p), nil
}

func (l *LokiLogger) Publish(t *testing.T) {
	t.Helper()
	t.Log("Pushing log entries to loki...")

	values := make([][]string, 0)

	l.mu.Lock()
	defer l.mu.Unlock()

	for k, v := range l.entries {
		values = append(values, []string{strconv.FormatInt(k, 10), v})
	}

	payload := map[string]interface{}{
		"streams": []map[string]interface{}{
			{
				"stream": map[string]string{
					"test":   t.Name(),
					"failed": strconv.FormatBool(t.Failed()),
				},
				"values": values,
			},
		},
	}

	marshalled, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal payload: %v", err)
		return
	}

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, l.LokiURL+"/loki/api/v1/push", bytes.NewReader(marshalled))
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("failed to push logs to loki: %v", err)
		return
	}

	if resp.StatusCode >= http.StatusBadRequest {
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("failed to read error response: %v", err)
			return
		}
		t.Logf("Error response: %s", string(body))
		t.Fatalf("failed to push logs to loki: %d", resp.StatusCode)
		return
	}

	t.Logf("Pushed %d log entries to loki", len(values))
}

// LokiContainerAndLogging starts or reuses the named Loki container on networkID
// and routes zerolog output through it for the rest of the test.
func LokiContainerAndLogging(t *testing.T, networkID string) (ConnectionString string) {
	t.Helper()

	ctx := context.TODO()

	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok, "Fail to locate current caller folder")

	dataFolder := path.Join(path.Dir(filename), "metrics/loki")

	container := testcontainers.ContainerRequest{
		Image:        "grafana/loki:latest",
		ExposedPorts: []string{"3100/tcp"},
		WaitingFor:   wait.ForLog("will now accept requests"),
		HostConfigModifier: func(hostConfig *mobycontainer.HostConfig) {
			hostConfig.Binds = []string{dataFolder + ":/loki"}
		},
		Networks: []string{networkID},
	}

	genericReq := testcontainers.GenericContainerRequest{
		ContainerRequest: container,
		Started:          true,
	}

	reuse := testcontainers.CustomizeRequestOption(
		func(req *testcontainers.GenericContainerRequest) error {
			req.Reuse = true
			req.Name = "foreverbull-testing-grafana-loki"
			return nil
		},
	)
	err := reuse.Customize(&genericReq)
	require.NoError(t, err)

	cont, err := testcontainers.GenericContainer(ctx, genericReq)
	require.NoError(t, err)
	host, err := cont.Host(ctx)
	require.NoError(t, err)
	port, err := cont.MappedPort(ctx, "3100/tcp")
	require.NoError(t, err)

	lokiLogger := &LokiLogger{entries: make(map[int64]string), LokiURL: fmt.Sprintf("http://%s:%d", host, port.Num())}

	t.Cleanup(func() {
		lokiLogger.Publish(t)
		require.NoError(t, cont.Terminate(ctx))
	})

	log.Logger = zerolog.New(zerolog.MultiLevelWriter(zerolog.NewConsoleWriter(), lokiLogger))

	return fmt.Sprintf("http://%s:%d", host, port.Num())
}

// ZiplineContainer starts or reuses the named Zipline container on network.
func ZiplineContainer(t *testing.T, network string) string {
	t.Helper()

	healthStrategy := wait.NewHealthStrategy()

	container := testcontainers.ContainerRequest{
		Image:      "lhjnilsson/zipline:local",
		WaitingFor: healthStrategy,
		Networks:   []string{network},
		Env:        map[string]string{"DATABASE_URL": environment.GetPostgresURL()},
	}
	genericReq := testcontainers.GenericContainerRequest{
		ContainerRequest: container,
		Started:          true,
	}

	reuse := testcontainers.CustomizeRequestOption(
		func(req *testcontainers.GenericContainerRequest) error {
			req.Reuse = true
			req.Name = "foreverbull-testing-zipline"
			return nil
		},
	)
	err := reuse.Customize(&genericReq)
	require.NoError(t, err)
	c, err := testcontainers.GenericContainer(context.TODO(), genericReq)
	require.NoError(t, err)

	t.Cleanup(func() {
		err := c.Terminate(context.TODO())
		require.NoError(t, err)
	})

	return "foreverbull-testing-zipline"
}
