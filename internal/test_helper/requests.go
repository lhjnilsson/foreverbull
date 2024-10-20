package test_helper

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type Service struct {
	Name        string
	CreatedAt   time.Time
	Image       string
	Status      string
	ServiceType *string
}

type Backtest struct {
	Name       string
	Status     string
	Service    string
	Calendar   string
	Start      time.Time
	End        time.Time
	Benchmark  string
	Symbols    []string
	IngestedAt *time.Time
}

type Strategy struct {
	Name     string
	Backtest string
}

func Request(t *testing.T, method string, endpoint string, payload interface{}) *http.Response {
	t.Helper()

	var err error

	var res *http.Response

	var req *http.Request

	ctx := context.Background()

	if payload != nil {
		str, isString := payload.(string)
		if isString {
			req, err = http.NewRequestWithContext(ctx, method, "http://localhost:8080"+endpoint, bytes.NewBufferString(str))
			require.NoError(t, err)
			req.Header.Set("Content-Type", "application/json")
		} else {
			marshalled, err := json.Marshal(payload)
			if err != nil {
				t.Fatalf("Failed to marshal payload: %v", err)
				return nil
			}

			bytes := bytes.NewReader(marshalled)
			req, err = http.NewRequestWithContext(ctx, method, "http://localhost:8080"+endpoint, bytes)
			require.NoError(t, err)
			req.Header.Set("Content-Type", "application/json")
		}
	} else {
		req, err = http.NewRequestWithContext(ctx, method, "http://localhost:8080"+endpoint, nil)
	}

	if err != nil {
		t.Fatalf("Error creating request: %v", err)
		return nil
	}

	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Error sending request: %v", err)
		return nil
	}

	return res
}

func CleanupEnv(t *testing.T, workerService Service, backtestService Service, backtest Backtest,
	strategy *Strategy,
) {
	t.Helper()

	// Delete old services
	rsp := Request(t, http.MethodDelete, "/service/api/services/"+workerService.Name, nil)
	require.NoError(t, rsp.Body.Close())
	rsp = Request(t, http.MethodDelete, "/service/api/services/"+backtestService.Name, nil)
	require.NoError(t, rsp.Body.Close())
	rsp = Request(t, http.MethodDelete, "/backtest/api/backtests/"+backtest.Name, nil)
	require.NoError(t, rsp.Body.Close())

	if strategy != nil {
		Request(t, http.MethodDelete, "/strategy/api/strategies/"+strategy.Name, nil)
	}
}

func SetUpEnv(t *testing.T, backtest Backtest, strategy *Strategy) error {
	t.Helper()
	// Create backtest
	rsp := Request(t, http.MethodPost, "/backtest/api/backtests", backtest)
	if !assert.Equal(t, http.StatusCreated, rsp.StatusCode) {
		rspData, _ := io.ReadAll(rsp.Body)
		t.Fatalf("Failed to create backtest: %s", string(rspData))
	}

	t.Logf("Backtest %s created", backtest.Name)

	backoff := time.Second / 2

	for i := 0; i <= 60; i++ {
		rsp := Request(t, http.MethodGet, "/backtest/api/backtests/"+backtest.Name, nil)
		if !assert.Equal(t, http.StatusOK, rsp.StatusCode) {
			rspData, _ := io.ReadAll(rsp.Body)
			t.Fatalf("Failed to get backtest: %s", string(rspData))
		}

		err := json.NewDecoder(rsp.Body).Decode(&backtest)
		if err != nil {
			t.Fatalf("Failed to decode backtest: %v", err)
		}

		if backtest.Status == "READY" {
			break
		} else if backtest.Status == "CREATED" {
			time.Sleep(backoff)
			continue
		} else {
			t.Fatalf("Backtest %s in error state: %s", backtest.Name, backtest.Status)
		}

		t.Fatalf("Backtest %s not ready after loop", backtest.Name)
	}

	if backtest.Status != "READY" {
		t.Fatalf("Backtest %s not ready", backtest.Name)
	}

	t.Logf("Backtest %s ready", backtest.Name)

	// Create strategy
	if strategy != nil {
		rsp := Request(t, http.MethodPost, "/strategy/api/strategies", strategy)
		if !assert.Equal(t, http.StatusCreated, rsp.StatusCode) {
			rspData, _ := io.ReadAll(rsp.Body)
			t.Fatalf("Failed to create strategy: %s", string(rspData))
		}

		err := json.NewDecoder(rsp.Body).Decode(&strategy)
		if err != nil {
			t.Fatalf("Failed to decode strategy: %v", err)
		}
	}

	return nil
}
