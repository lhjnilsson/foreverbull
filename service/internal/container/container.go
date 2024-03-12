package container

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"strings"

	def "github.com/lhjnilsson/foreverbull/service/container"
	"github.com/rs/zerolog/log"

	cType "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/lhjnilsson/foreverbull/internal/environment"
)

func NewContainerRegistry() (def.Container, error) {
	client, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("error creating docker client: %v", err)
	}
	return &container{client: client}, nil
}

type container struct {
	client *client.Client
}

func (sc *container) Start(ctx context.Context, image, name string, extraLabels map[string]string) (string, error) {
	env := []string{fmt.Sprintf("BROKER_HOSTNAME=%s", environment.GetServerAddress())}
	env = append(env, fmt.Sprintf("BROKER_HTTP_PORT=%s", environment.GetHTTPPort()))
	env = append(env, fmt.Sprintf("STORAGE_ENDPOINT=%s", environment.GetMinioURL()))
	env = append(env, fmt.Sprintf("STORAGE_ACCESS_KEY=%s", environment.GetMinioAccessKey()))
	env = append(env, fmt.Sprintf("STORAGE_SECRET_KEY=%s", environment.GetMinioSecretKey()))
	env = append(env, fmt.Sprintf("DATABASE_URL=%s", environment.GetPostgresURL()))
	env = append(env, fmt.Sprintf("LOGLEVEL=%s", environment.GetLogLevel()))

	labels := map[string]string{"platform": "foreverbull", "type": "service"}
	for k, v := range extraLabels {
		labels[k] = v
	}

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

	resp, err := sc.client.ContainerCreate(ctx, &conf, &hostConf, &networkConfig, nil, name)
	if err != nil {
		return "", fmt.Errorf("error creating container: %v", err)
	}
	err = sc.client.ContainerStart(ctx, resp.ID, cType.StartOptions{})
	if err != nil {
		return "", fmt.Errorf("error starting container: %v", err)
	}

	logs, err := sc.client.ContainerLogs(ctx, resp.ID, cType.LogsOptions{ShowStdout: true, ShowStderr: true, Follow: true})
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
			log.Debug().Str("container", resp.ID).Str("image", image).Msg(string(message))
		}
	}()
	return resp.ID[:12], nil
}

func (sc *container) SaveImage(ctx context.Context, containerID, imageName string) error {
	_, err := sc.client.ContainerCommit(ctx, containerID, cType.CommitOptions{Reference: imageName})
	return err
}

func (sc *container) Stop(ctx context.Context, containerID string, remove bool) error {
	if err := sc.client.ContainerStop(ctx, containerID, cType.StopOptions{}); err != nil {
		return err
	}
	if remove {
		return sc.client.ContainerRemove(ctx, containerID, cType.RemoveOptions{})
	}
	return nil
}

func (sc *container) StopAll(ctx context.Context, remove bool) error {
	filters := filters.NewArgs()
	filters.Add("label", "platform=foreverbull")
	filters.Add("label", "type=service")
	filters.Add("network", environment.GetDockerNetworkName())
	containers, err := sc.client.ContainerList(ctx, cType.ListOptions{All: true, Filters: filters})
	if err != nil {
		return fmt.Errorf("error listing containers: %v", err)
	}
	for _, c := range containers {
		log.Info().Str("id", c.ID).Bool("remove", remove).Msg("stopping container")
		if err := sc.Stop(ctx, c.ID, remove); err != nil {
			if strings.Contains(err.Error(), "No such container") {
				continue
			}
			return fmt.Errorf("error stopping container: %v", err)
		}
	}
	containers, err = sc.client.ContainerList(ctx, cType.ListOptions{All: true, Filters: filters})
	if err != nil {
		return fmt.Errorf("error listing images: %v", err)
	}
	if len(containers) == 0 {
		return nil
	}
	return fmt.Errorf("expected no containers, but found %d", len(containers))
}
