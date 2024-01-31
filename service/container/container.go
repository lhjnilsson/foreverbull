package container

import (
	"context"

	"github.com/docker/docker/api/types"
	"github.com/lhjnilsson/foreverbull/internal/config"
)

type Container interface {
	Info(ctx context.Context, containerID string) (types.ImageInspect, error)
	Pull(ctx context.Context, imageID string) error
	Start(ctx context.Context, config *config.Config, serviceName, image, containerID string) (string, error)
	SaveImage(ctx context.Context, containerID, name string) error
	Stop(ctx context.Context, containerID string, remove bool) error
}
