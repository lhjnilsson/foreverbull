package grpc

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/nats-io/nats.go"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc/codes"
	healthgrpc "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
)

const healthTimeout = 2 * time.Second

// HealthCheck reports SERVING only when Postgres, NATS, and Minio all respond.
type HealthCheck struct {
	healthgrpc.UnimplementedHealthServer

	postgres func(context.Context) error
	nats     func(context.Context) error
	minio    func(context.Context) error
}

func newHealthCheck(pool *pgxpool.Pool) *HealthCheck {
	return &HealthCheck{
		postgres: pool.Ping,
		nats:     pingNATS,
		minio:    pingMinio,
	}
}

func (h *HealthCheck) Check(ctx context.Context, _ *healthgrpc.HealthCheckRequest) (*healthgrpc.HealthCheckResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, healthTimeout)
	defer cancel()

	for _, c := range []struct {
		name string
		ping func(context.Context) error
	}{
		{"postgres", h.postgres},
		{"nats", h.nats},
		{"minio", h.minio},
	} {
		if c.ping == nil {
			log.Error().Str("dependency", c.name).Msg("health check is not configured")
			return &healthgrpc.HealthCheckResponse{Status: healthgrpc.HealthCheckResponse_NOT_SERVING}, nil
		}

		if err := c.ping(ctx); err != nil {
			log.Error().Err(err).Str("dependency", c.name).Msg("health check failed")
			return &healthgrpc.HealthCheckResponse{Status: healthgrpc.HealthCheckResponse_NOT_SERVING}, nil
		}
	}

	return &healthgrpc.HealthCheckResponse{Status: healthgrpc.HealthCheckResponse_SERVING}, nil
}

func (h *HealthCheck) Watch(_ *healthgrpc.HealthCheckRequest, _ healthgrpc.Health_WatchServer) error {
	return status.Error(codes.Unimplemented, "Watch is not implemented")
}

func pingNATS(ctx context.Context) error {
	nc, err := nats.Connect(environment.GetNATSURL(),
		nats.Name("foreverbull-health"),
		nats.Timeout(healthTimeout),
		nats.MaxReconnects(0),
	)
	if err != nil {
		return fmt.Errorf("nats connect: %w", err)
	}
	defer nc.Close()

	if err := nc.FlushWithContext(ctx); err != nil {
		return fmt.Errorf("nats flush: %w", err)
	}

	return nil
}

func pingMinio(ctx context.Context) error {
	url := "http://" + environment.GetMinioURL() + "/minio/health/live"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("minio health request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("minio health: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	_, _ = io.Copy(io.Discard, resp.Body)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("minio health: status %s", resp.Status)
	}

	return nil
}
