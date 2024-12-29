package container

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"

	"github.com/docker/docker/api/types"
	cType "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/rs/zerolog/log"
)

const (
	DockerHeaderSize = 8
)

type Container interface {
	GetStatus() (string, error)
	GetHealth() (string, error)
	GetIpAddress() (string, error)
	GetConnectionString() (string, error)
	Stop() error
}

type container struct {
	client *client.Client

	container types.ContainerJSON
}

func GetContainer(ctx context.Context, containerID string) (Container, error) {
	client, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("error creating docker client: %w", err)
	}

	cont, err := client.ContainerInspect(ctx, containerID)
	if err != nil {
		return nil, fmt.Errorf("error inspecting container: %w", err)
	}

	return &container{client: client, container: cont}, nil
}

func (c *container) GetStatus() (string, error) {
	container, err := c.client.ContainerInspect(context.Background(), c.container.ID)
	if err != nil {
		return "", fmt.Errorf("error inspecting container: %w", err)
	}

	return container.State.Status, nil
}

func (c *container) GetHealth() (string, error) {
	container, err := c.client.ContainerInspect(context.Background(), c.container.ID)
	if err != nil {
		return "", fmt.Errorf("error inspecting container: %w", err)
	}

	if container.State.Health == nil {
		return "", errors.New("container has no health")
	}

	return container.State.Health.Status, nil
}

func (c *container) GetIpAddress() (string, error) {
	container, err := c.client.ContainerInspect(context.Background(), c.container.ID)
	if err != nil {
		return "", fmt.Errorf("error inspecting container: %w", err)
	}
	return container.NetworkSettings.Networks[environment.GetDockerNetworkName()].IPAddress, nil
}

func (c *container) GetConnectionString() (string, error) {
	container, err := c.client.ContainerInspect(context.Background(), c.container.ID)
	if err != nil {
		return "", fmt.Errorf("error inspecting container: %w", err)
	}

	return fmt.Sprintf("%s:%d", container.NetworkSettings.Networks[environment.GetDockerNetworkName()].IPAddress, 50055), nil
}

func (c *container) Stop() error {
	err := c.client.ContainerStop(context.Background(), c.container.ID, cType.StopOptions{})
	if err != nil {
		return fmt.Errorf("error stopping container: %w", err)
	}

	err = c.client.ContainerRemove(context.Background(), c.container.ID, cType.RemoveOptions{})
	if err != nil {
		return fmt.Errorf("error removing container: %w", err)
	}

	return nil
}

func NewEngine() (Engine, error) {
	client, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("error creating docker client: %w", err)
	}

	return &engine{client: client}, nil
}

type Engine interface {
	PullImage() error

	Start(ctx context.Context, image, name string) (Container, error)
	StopAll(ctx context.Context, remove bool) error
}

type engine struct {
	client *client.Client
}

func (e *engine) PullImage() error {
	return nil
}

func (e *engine) Start(ctx context.Context, image string, name string) (Container, error) {
	env := []string{"BROKER_HOSTNAME=" + environment.GetServerAddress()}
	env = append(env, "BROKER_PORT="+environment.GetHTTPPort())
	env = append(env, "STORAGE_ENDPOINT="+environment.GetMinioURL())
	env = append(env, "STORAGE_ACCESS_KEY="+environment.GetMinioAccessKey())
	env = append(env, "STORAGE_SECRET_KEY="+environment.GetMinioSecretKey())
	env = append(env, "DATABASE_URL="+environment.GetPostgresURL())
	env = append(env, "LOGLEVEL="+environment.GetLogLevel())

	labels := map[string]string{"platform": "foreverbull", "type": "service"}

	conf := cType.Config{Image: image, Env: env, Tty: false, Hostname: name, Labels: labels}
	hostConf := cType.HostConfig{
		ExtraHosts: []string{"host.docker.internal:host-gateway"},
	}

	networkConfig := network.NetworkingConfig{
		EndpointsConfig: map[string]*network.EndpointSettings{},
	}
	endpointSettings := &network.EndpointSettings{
		NetworkID: environment.GetDockerNetworkName(),
	}

	networkConfig.EndpointsConfig[environment.GetDockerNetworkName()] = endpointSettings

	resp, err := e.client.ContainerCreate(ctx, &conf, &hostConf, &networkConfig, nil, name)
	if err != nil {
		return nil, fmt.Errorf("error creating container: %w", err)
	}

	err = e.client.ContainerStart(ctx, resp.ID, cType.StartOptions{})
	if err != nil {
		return nil, fmt.Errorf("error starting container: %w", err)
	}

	logs, err := e.client.ContainerLogs(ctx, resp.ID, cType.LogsOptions{ShowStdout: true, ShowStderr: true, Follow: true})
	if err != nil {
		return nil, fmt.Errorf("error getting container logs: %w", err)
	}

	go func() {
		header := make([]byte, DockerHeaderSize)

		for {
			_, err := logs.Read(header)
			if errors.Is(err, io.EOF) {
				logs.Close()
				break
			}
			if err != nil {
				log.Err(err).Msg("error reading container logs")
				return
			}

			if err != nil {
				log.Error().Err(err).Msg("error reading container logs")
			}

			count := binary.BigEndian.Uint32(header[4:])
			if count == 0 {
				continue
			}

			message := make([]byte, count)

			_, err = logs.Read(message)
			if errors.Is(err, io.EOF) {
				logs.Close()

				break
			}

			if err != nil {
				log.Error().Err(err).Msg("error reading container logs")
			}

			log.Debug().Str("container", resp.ID).Str("image", image).Msg(string(message))
		}
	}()

	return GetContainer(ctx, resp.ID)
}

func (e *engine) StopAll(ctx context.Context, _ bool) error {
	filters := filters.NewArgs()
	filters.Add("label", "platform=foreverbull")
	filters.Add("label", "type=service")
	filters.Add("network", environment.GetDockerNetworkName())

	containers, err := e.client.ContainerList(ctx, cType.ListOptions{All: true, Filters: filters})
	if err != nil {
		return fmt.Errorf("error listing containers: %w", err)
	}

	for _, c := range containers {
		c, err := GetContainer(ctx, c.ID)
		if err != nil {
			return fmt.Errorf("error getting container: %w", err)
		}

		if err := c.Stop(); err != nil {
			return fmt.Errorf("error stopping container: %w", err)
		}
	}

	containers, err = e.client.ContainerList(ctx, cType.ListOptions{All: true, Filters: filters})
	if err != nil {
		return fmt.Errorf("error listing images: %w", err)
	}

	if len(containers) == 0 {
		return nil
	}

	return fmt.Errorf("expected no containers, but found %d", len(containers))
}
