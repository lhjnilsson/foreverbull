package container

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/docker/docker/api/types"
	def "github.com/lhjnilsson/foreverbull/service/container"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/lhjnilsson/foreverbull/internal/environment"
)

func New() (def.Container, error) {
	client, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("error creating docker client: %v", err)
	}
	return &serviceContainer{client: client}, nil
}

type serviceContainer struct {
	client *client.Client
}

func (sc *serviceContainer) hasImage(ctx context.Context, imageID string) error {
	_, _, err := sc.client.ImageInspectWithRaw(ctx, imageID)
	if err != nil && strings.Contains(err.Error(), "No such image: ") {
		return errors.New("no such image")
	}
	return err
}

func (sc *serviceContainer) Pull(ctx context.Context, imageID string) error {
	reader, err := sc.client.ImagePull(ctx, imageID, types.ImagePullOptions{})
	if err != nil {
		return err
	}
	_, err = io.Copy(io.Discard, reader)
	return err
}

type ContainerStatus struct {
	ID  string
	Err error
}

func (sc *serviceContainer) Info(ctx context.Context, containerID string) (types.ImageInspect, error) {
	i, _, err := sc.client.ImageInspectWithRaw(ctx, containerID)
	return i, err
}

func (sc *serviceContainer) Start(ctx context.Context, serviceName, image, name string, extraLabels map[string]string) (string, error) {
	if err := sc.hasImage(ctx, image); err != nil && err.Error() == "no such image" {
		if err := sc.Pull(ctx, image); err != nil {
			return "", fmt.Errorf("error pulling image '%s': %v", image, err)
		}
	} else if err != nil {
		return "", fmt.Errorf("error inspecting image: %v", err)
	}
	env := []string{fmt.Sprintf("BROKER_HOSTNAME=%s", environment.GetServerAddress())}
	env = append(env, fmt.Sprintf("BROKER_HTTP_PORT=%s", environment.GetHTTPPort()))
	env = append(env, fmt.Sprintf("SERVICE_NAME=%s", serviceName))
	env = append(env, fmt.Sprintf("STORAGE_ENDPOINT=%s", environment.GetMinioURL()))
	env = append(env, fmt.Sprintf("STORAGE_ACCESS_KEY=%s", environment.GetMinioAccessKey()))
	env = append(env, fmt.Sprintf("STORAGE_SECRET_KEY=%s", environment.GetMinioSecretKey()))
	env = append(env, fmt.Sprintf("DATABASE_URL=%s", environment.GetPostgresURL()))
	env = append(env, fmt.Sprintf("LOGLEVEL=%s", environment.GetLogLevel()))

	labels := map[string]string{"platform": "foreverbull", "type": "service", "service": serviceName}
	for k, v := range extraLabels {
		labels[k] = v
	}

	conf := container.Config{Image: image, Env: env, Tty: false, Hostname: name, Labels: labels}
	hostConf := container.HostConfig{
		ExtraHosts: []string{"host.docker.internal:host-gateway"},
	}

	networkConfig := network.NetworkingConfig{
		EndpointsConfig: map[string]*network.EndpointSettings{},
	}
	endpointSettings := &network.EndpointSettings{
		NetworkID: environment.GetDockerNetworkName(),
	}

	networkConfig.EndpointsConfig[environment.GetDockerNetworkName()] = endpointSettings

	resp, err := sc.client.ContainerCreate(ctx, &conf, &hostConf, &networkConfig, nil, name)
	if err != nil {
		return "", fmt.Errorf("error creating container: %v", err)
	}
	err = sc.client.ContainerStart(ctx, resp.ID, container.StartOptions{})
	if err != nil {
		return "", fmt.Errorf("error starting container: %v", err)
	}
	return resp.ID[:12], nil
}

func (sc *serviceContainer) SaveImage(ctx context.Context, containerID, imageName string) error {
	_, err := sc.client.ContainerCommit(ctx, containerID, container.CommitOptions{Reference: imageName})
	return err
}

func (sc *serviceContainer) Stop(ctx context.Context, containerID string, remove bool) error {
	if err := sc.client.ContainerStop(ctx, containerID, container.StopOptions{}); err != nil {
		return err
	}
	if remove {
		return sc.client.ContainerRemove(ctx, containerID, container.RemoveOptions{})
	}
	return nil
}
