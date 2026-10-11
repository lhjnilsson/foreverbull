package grpc

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/stretchr/testify/require"
	healthgrpc "google.golang.org/grpc/health/grpc_health_v1"
)

func okPing(context.Context) error { return nil }

func failPing(context.Context) error { return errors.New("unavailable") }

func TestHealthCheckServingWhenAllDepsOk(t *testing.T) {
	t.Parallel()

	h := &HealthCheck{postgres: okPing, nats: okPing, minio: okPing}
	resp, err := h.Check(context.Background(), &healthgrpc.HealthCheckRequest{})
	require.NoError(t, err)
	require.Equal(t, healthgrpc.HealthCheckResponse_SERVING, resp.Status)
}

func TestHealthCheckNotServingWhenDepFails(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		h    *HealthCheck
	}{
		{"postgres", &HealthCheck{postgres: failPing, nats: okPing, minio: okPing}},
		{"nats", &HealthCheck{postgres: okPing, nats: failPing, minio: okPing}},
		{"minio", &HealthCheck{postgres: okPing, nats: okPing, minio: failPing}},
		{"missing pinger", &HealthCheck{postgres: okPing, nats: okPing}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			resp, err := tc.h.Check(context.Background(), &healthgrpc.HealthCheckRequest{})
			require.NoError(t, err)
			require.Equal(t, healthgrpc.HealthCheckResponse_NOT_SERVING, resp.Status)
		})
	}
}

func TestPingMinioLiveEndpoint(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)
	t.Setenv(environment.MinioURL, strings.TrimPrefix(srv.URL, "http://"))

	require.NoError(t, pingMinio(context.Background()))
	require.Equal(t, "/minio/health/live", gotPath)
}

func TestPingMinioUnhealthyStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	t.Cleanup(srv.Close)
	t.Setenv(environment.MinioURL, strings.TrimPrefix(srv.URL, "http://"))

	require.Error(t, pingMinio(context.Background()))
}

func TestPingNATSUnreachable(t *testing.T) {
	t.Setenv(environment.NatsURL, "nats://127.0.0.1:1")
	require.Error(t, pingNATS(context.Background()))
}
