package container

import "context"

const (
	// HealthHealthy is the GetHealth value callers treat as ready
	// (matches Docker's types.Healthy used by pkg/backtest).
	HealthHealthy   = "healthy"
	HealthUnhealthy = "unhealthy"
	HealthStarting  = "starting"

	LabelPlatform = "platform"
	LabelType     = "type"
	LabelAppName  = "app.kubernetes.io/name"

	LabelPlatformValue = "foreverbull"
	LabelTypeService   = "service"

	grpcPort = 50055
)

type Container interface {
	GetStatus() (string, error)
	GetHealth() (string, error)
	GetIpAddress() (string, error)
	GetConnectionString() (string, error)
	Stop() error
}

type Engine interface {
	Get(ctx context.Context, name string) (Container, error)
	Start(ctx context.Context, image, name string) (Container, error)
	StopAll(ctx context.Context, remove bool) error
}
