package dependency

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	bs "github.com/lhjnilsson/foreverbull/backtest/stream"
	"github.com/lhjnilsson/foreverbull/internal/stream"
	"github.com/lhjnilsson/foreverbull/service/backtest/engine"
	service "github.com/lhjnilsson/foreverbull/service/entity"
)

/*
TODO: Move this to a proper http client
*/

const GetHTTPClientKey stream.Dependency = "get_http_client"

type HTTPClient struct {
	BaseURL string
	Client  *http.Client
}

func (c *HTTPClient) Get(ctx context.Context, url string, v any) error {
	rsp, err := c.Client.Get(c.BaseURL + url)
	if err != nil {
		return fmt.Errorf("error getting %s: %w", url, err)
	}
	if rsp.StatusCode != 200 {
		return fmt.Errorf("error getting %s: %s", url, rsp.Status)
	}
	if v == nil {
		return nil
	}
	err = json.NewDecoder(rsp.Body).Decode(v)
	if err != nil {
		return fmt.Errorf("error decoding %s: %w", url, err)
	}
	return nil
}

func GetHTTPClient() *HTTPClient {
	return &HTTPClient{
		BaseURL: "http://localhost:8080/",
		Client:  http.DefaultClient,
	}
}

const GetBacktestEngineKey stream.Dependency = "get_backtest_engine"

func GetBacktestEngine(ctx context.Context, message stream.Message) (interface{}, error) {
	command := bs.BacktestIngestCommand{}
	err := message.ParsePayload(&command)
	if err != nil {
		return nil, fmt.Errorf("error unmarshalling MarketdataDownloaded payload: %w", err)
	}

	http := message.MustGet(GetHTTPClientKey).(*HTTPClient)
	var instance service.Instance
	err = http.Get(ctx, "service/api/instances/"+command.ServiceInstanceID, &instance)
	if err != nil {
		return nil, fmt.Errorf("error getting instances: %w", err)
	}
	return engine.NewZiplineEngine(ctx, &instance)
}
