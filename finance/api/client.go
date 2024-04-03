package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/hashicorp/go-retryablehttp"
)

type Client interface {
	GetPortfolio(ctx context.Context) (*GetPortfolioResponse, error)
}

type client struct {
	client *retryablehttp.Client

	baseURL string
}

func NewClient() (Client, error) {
	cl := retryablehttp.NewClient()
	return &client{
		client:  cl,
		baseURL: "http://localhost:8080/finance/api",
	}, nil
}

func (c *client) GetPortfolio(ctx context.Context) (*GetPortfolioResponse, error) {
	req, err := c.client.Get(c.baseURL + "/portfolio")
	if err != nil {
		return nil, err
	}
	if req.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", req.StatusCode)
	}
	var portfolio GetPortfolioResponse
	err = json.NewDecoder(req.Body).Decode(&portfolio)
	if err != nil {
		return nil, fmt.Errorf("error decoding response: %w", err)
	}
	return &portfolio, nil
}
