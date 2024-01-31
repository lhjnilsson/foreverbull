package container

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"strings"

	"github.com/docker/docker/api/types"
	def "github.com/lhjnilsson/foreverbull/service/container"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/lhjnilsson/foreverbull/internal/config"
	"go.uber.org/zap"
)

func New(log *zap.Logger) (def.Container, error) {
	client, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("error creating docker client: %v", err)
	}
	return &serviceContainer{log: log, client: client}, nil
}

type serviceContainer struct {
	log    *zap.Logger
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
	_, err = io.Copy(ioutil.Discard, reader)
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

func (sc *serviceContainer) Start(ctx context.Context, config *config.Config, serviceName, image, name string) (string, error) {
	if err := sc.hasImage(ctx, image); err != nil && err.Error() == "no such image" {
		sc.log.Debug("pulling image", zap.String("image", image))
		if err := sc.Pull(ctx, image); err != nil {
			return "", fmt.Errorf("error pulling image '%s': %v", image, err)
		}
	} else if err != nil {
		return "", fmt.Errorf("error inspecting image: %v", err)
	}
	sc.log.Debug("starting container", zap.String("image", image), zap.String("service", serviceName))

	env := []string{fmt.Sprintf("BROKER_HOSTNAME=%s", config.Hostname)}
	env = append(env, fmt.Sprintf("BROKER_HTTP_PORT=%d", config.HTTP.Port))
	env = append(env, fmt.Sprintf("SERVICE_NAME=%s", serviceName))
	env = append(env, fmt.Sprintf("STORAGE_ENDPOINT=%s", config.MinioURI))
	env = append(env, fmt.Sprintf("STORAGE_ACCESS_KEY=%s", config.MinioAccessKey))
	env = append(env, fmt.Sprintf("STORAGE_SECRET_KEY=%s", config.MinioSecretKey))
	env = append(env, fmt.Sprintf("DATABASE_URL=%s", config.PostgresURI))
	env = append(env, fmt.Sprintf("LOGLEVEL=%s", config.ClientLogLevel))

	conf := container.Config{Image: image, Env: env, Tty: false, Hostname: name,
		Labels: map[string]string{"platform": "foreverbull", "type": "service", "service": serviceName}}
	hostConf := container.HostConfig{
		ExtraHosts: []string{"host.docker.internal:host-gateway"},
	}

	networkConfig := network.NetworkingConfig{
		EndpointsConfig: map[string]*network.EndpointSettings{},
	}
	endpointSettings := &network.EndpointSettings{
		NetworkID: config.Docker.Network,
	}

	networkConfig.EndpointsConfig[config.Docker.Network] = endpointSettings

	resp, err := sc.client.ContainerCreate(ctx, &conf, &hostConf, &networkConfig, nil, name)
	if err != nil {
		return "", fmt.Errorf("error creating container: %v", err)
	}
	err = sc.client.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{})
	if err != nil {
		return "", fmt.Errorf("error starting container: %v", err)
	}
	return resp.ID[:12], nil
}

func (sc *serviceContainer) SaveImage(ctx context.Context, containerID, imageName string) error {
	_, err := sc.client.ContainerCommit(ctx, containerID, types.ContainerCommitOptions{Reference: imageName})
	return err
}

func (sc *serviceContainer) Stop(ctx context.Context, containerID string, remove bool) error {
	if err := sc.client.ContainerStop(ctx, containerID, container.StopOptions{}); err != nil {
		return err
	}
	if remove {
		return sc.client.ContainerRemove(ctx, containerID, types.ContainerRemoveOptions{})
	}
	return nil
}
