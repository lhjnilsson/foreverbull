package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"os"
	"os/signal"
	"path"
	"runtime"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
)

const NETWORKID = "foreverbull_metrics"

func main() {
	client, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		panic(err)
	}
	client.NetworkCreate(context.Background(), NETWORKID, types.NetworkCreate{CheckDuplicate: true})

	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		panic("Fail to locate current caller folder")
	}
	dataPath := path.Join(path.Dir(filename), "/loki")

	conf := container.Config{Image: "grafana/loki:latest", ExposedPorts: map[nat.Port]struct{}{"3100/tcp": {}}}
	hConfig := container.HostConfig{
		Binds: []string{dataPath + ":/loki"},
		PortBindings: nat.PortMap{
			"3100/tcp": []nat.PortBinding{{HostIP: "0.0.0.0", HostPort: "3100"}},
		},
	}
	nConfig := network.NetworkingConfig{
		EndpointsConfig: map[string]*network.EndpointSettings{NETWORKID: {
			Aliases:   []string{"loki"},
			NetworkID: NETWORKID,
		}},
	}
	resp, err := client.ContainerCreate(context.Background(), &conf, &hConfig, &nConfig, nil, "loki")
	if err != nil {
		panic(err)
	}
	if err := client.ContainerStart(context.Background(), resp.ID, container.StartOptions{}); err != nil {
		panic(err)
	}
	lokiID := resp.ID

	conf = container.Config{Image: "grafana/grafana-enterprise:latest", ExposedPorts: map[nat.Port]struct{}{"3000/tcp": {}}}
	hConfig = container.HostConfig{
		Binds: []string{"grafana:/var/lib/grafana"},
		PortBindings: nat.PortMap{
			"3000/tcp": []nat.PortBinding{{HostIP: "0.0.0.0", HostPort: "3000"}},
		},
	}
	nConfig = network.NetworkingConfig{
		EndpointsConfig: map[string]*network.EndpointSettings{NETWORKID: {
			Aliases:   []string{"grafana"},
			NetworkID: NETWORKID,
		}},
	}
	resp, err = client.ContainerCreate(context.Background(), &conf, &hConfig, &nConfig, nil, "grafana")
	if err != nil {
		panic(err)
	}
	if err := client.ContainerStart(context.Background(), resp.ID, container.StartOptions{}); err != nil {
		panic(err)
	}

	datasource := map[string]interface{}{
		"name":      "loki",
		"type":      "loki",
		"url":       "http://loki:3100",
		"access":    "proxy",
		"basicAuth": false,
	}
	payload, err := json.Marshal(datasource)
	if err != nil {
		panic(err)
	}
	time.Sleep(time.Second * 2)
	dreq, err := http.NewRequest("POST", "http://admin:admin@localhost:3000/api/datasources", bytes.NewReader(payload))
	dreq.Header.Set("Content-Type", "application/json")
	_, err = http.DefaultClient.Do(dreq)
	if err != nil {
		panic(err)
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	<-c

	client.ContainerStop(context.Background(), resp.ID, container.StopOptions{})
	client.ContainerRemove(context.Background(), resp.ID, container.RemoveOptions{})
	client.ContainerStop(context.Background(), lokiID, container.StopOptions{})
	client.ContainerRemove(context.Background(), lokiID, container.RemoveOptions{})
}
