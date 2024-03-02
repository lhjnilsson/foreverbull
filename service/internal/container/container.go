package container

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"strings"

	"github.com/docker/docker/api/types"
	def "github.com/lhjnilsson/foreverbull/service/container"
	"github.com/rs/zerolog/log"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
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

func (sc *serviceContainer) hasImage(ctx context.Context, imageID string) (bool, error) {
	_, _, err := sc.client.ImageInspectWithRaw(ctx, imageID)
	if err != nil {
		if strings.Contains(err.Error(), "No such image: ") {
			return false, nil
		} else {
			return false, err
		}
	}
	return true, nil
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
	has, err := sc.hasImage(ctx, image)
	if err != nil {
		return "", fmt.Errorf("error inspecting image: %v", err)
	}
	if !has {
		if err := sc.Pull(ctx, image); err != nil {
			return "", fmt.Errorf("error pulling image '%s': %v", image, err)
		}
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

	logs, err := sc.client.ContainerLogs(ctx, resp.ID, container.LogsOptions{ShowStdout: true, ShowStderr: true, Follow: true})
	if err != nil {
		return "", fmt.Errorf("error getting container logs: %v", err)
	}
	go func() {
		header := make([]byte, 8)
		for {
			_, err := logs.Read(header)
			if err == io.EOF {
				logs.Close()
				break
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
			if err == io.EOF {
				logs.Close()
				break
			}
			if err != nil {
				log.Error().Err(err).Msg("error reading container logs")
			}
			log.Debug().Str("container", resp.ID).Str("service", serviceName).Msg(string(message))
		}
	}()
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

func (sc *serviceContainer) StopAll(ctx context.Context, remove bool) error {
	filters := filters.NewArgs()
	filters.Add("label", "platform=foreverbull")
	filters.Add("label", "type=service")
	filters.Add("network", environment.GetDockerNetworkName())
	images, err := sc.client.ContainerList(ctx, container.ListOptions{All: true, Filters: filters})
	if err != nil {
		return fmt.Errorf("error listing containers: %v", err)
	}
	for _, image := range images {
		log.Info().Str("id", image.ID).Bool("remove", remove).Msg("stopping container")
		if err := sc.Stop(ctx, image.ID, remove); err != nil {
			return fmt.Errorf("error stopping container: %v", err)
		}
	}
	return err
}
