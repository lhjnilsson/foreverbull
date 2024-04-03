package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/hashicorp/go-retryablehttp"
)

type Client interface {
	ListServices(ctx context.Context) (*[]ServiceResponse, error)
	GetService(ctx context.Context, image string) (*ServiceResponse, error)

	ListInstances(ctx context.Context, image string) (*[]InstanceResponse, error)
	GetInstance(ctx context.Context, InstanceID string) (*InstanceResponse, error)

	GetImage(ctx context.Context, image string) (*ImageResponse, error)
	DownloadImage(ctx context.Context, image string) (*ImageResponse, error)
}

func NewClient() (Client, error) {
	cl := retryablehttp.NewClient()
	return &client{
		client:  cl,
		baseURL: "http://localhost:8080/service/api",
	}, nil
}

type client struct {
	client *retryablehttp.Client

	baseURL string
}

func (c *client) ListServices(ctx context.Context) (*[]ServiceResponse, error) {
	req, err := c.client.Get(c.baseURL + "/services")
	if err != nil {
		return nil, err
	}
	if req.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", req.StatusCode)
	}
	var services []ServiceResponse
	err = json.NewDecoder(req.Body).Decode(&services)
	if err != nil {
		return nil, fmt.Errorf("error decoding response: %w", err)
	}
	return &services, nil
}

func (c *client) GetService(ctx context.Context, name string) (*ServiceResponse, error) {
	req, err := c.client.Get(c.baseURL + "/services/" + name)
	if err != nil {
		return nil, err
	}
	if req.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", req.StatusCode)
	}
	var service ServiceResponse
	err = json.NewDecoder(req.Body).Decode(&service)
	if err != nil {
		return nil, fmt.Errorf("error decoding response: %w", err)
	}
	return &service, nil
}

func (c *client) ListInstances(ctx context.Context, image string) (*[]InstanceResponse, error) {
	req, err := c.client.Get(c.baseURL + "/instances?" + "image=" + image)
	if err != nil {
		return nil, err
	}
	if req.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", req.StatusCode)
	}
	var instances []InstanceResponse
	err = json.NewDecoder(req.Body).Decode(&instances)
	if err != nil {
		return nil, err
	}
	return &instances, nil
}

func (c *client) GetInstance(ctx context.Context, InstanceID string) (*InstanceResponse, error) {
	req, err := c.client.Get(c.baseURL + "/instances/" + InstanceID)
	if err != nil {
		return nil, err
	}
	if req.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", req.StatusCode)
	}
	var instance InstanceResponse
	err = json.NewDecoder(req.Body).Decode(&instance)
	if err != nil {
		return nil, err
	}
	return &instance, nil
}

func (c *client) GetImage(ctx context.Context, image string) (*ImageResponse, error) {
	req, err := c.client.Get(c.baseURL + "/images/" + image)
	if err != nil {
		return nil, err
	}
	if req.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", req.StatusCode)
	}
	var img ImageResponse
	err = json.NewDecoder(req.Body).Decode(&img)
	if err != nil {
		return nil, err
	}
	return &img, nil
}

func (c *client) DownloadImage(ctx context.Context, image string) (*ImageResponse, error) {
	req, err := c.client.Post(c.baseURL+"/images/"+image, "application/json", nil)
	if err != nil {
		return nil, err
	}
	if req.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("unexpected status code: %d", req.StatusCode)
	}
	var img ImageResponse
	err = json.NewDecoder(req.Body).Decode(&img)
	if err != nil {
		return nil, err
	}
	return &img, nil
}
