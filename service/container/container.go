package container

import (
	"context"

	"github.com/docker/docker/api/types"
)

type Container interface {
	Info(ctx context.Context, containerID string) (types.ImageInspect, error)
	Pull(ctx context.Context, imageID string) error
	Start(ctx context.Context, serviceName, image, containerID string, extraLabels map[string]string) (string, error)
	SaveImage(ctx context.Context, containerID, name string) error
	Stop(ctx context.Context, containerID string, remove bool) error
}
