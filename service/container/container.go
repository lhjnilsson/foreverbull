package container

import (
	"context"

	"github.com/lhjnilsson/foreverbull/service/entity"
)

type Container interface {
	Start(ctx context.Context, image, containerID string, extraLabels map[string]string) (string, error)
	SaveImage(ctx context.Context, containerID, name string) error
	Stop(ctx context.Context, containerID string, remove bool) error
	StopAll(ctx context.Context, remove bool) error
}

type Image interface {
	Info(ctx context.Context, name string) (*entity.Image, error)
	Pull(ctx context.Context, name string) (*entity.Image, error)
	Remove(ctx context.Context, name string) error
}
