package container

import (
	"context"
	"fmt"
	"io"
	"time"

	dockerImage "github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	def "github.com/lhjnilsson/foreverbull/pkg/service/container"
	"github.com/lhjnilsson/foreverbull/pkg/service/entity"
)

func NewImageRegistry() (def.Image, error) {
	client, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("error creating docker client: %v", err)
	}
	return &image{
		client: client,
	}, nil
}

type image struct {
	client *client.Client
}

func (si *image) Info(ctx context.Context, name string) (*entity.Image, error) {
	i, _, err := si.client.ImageInspectWithRaw(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("error inspecting image: %v", err)
	}
	created, err := time.Parse(time.RFC3339, i.Created)
	if err != nil {
		return nil, fmt.Errorf("error parsing created time: %v", err)
	}
	return &entity.Image{
		ID:        i.ID,
		Tags:      i.RepoTags,
		CreatedAt: created,
		Size:      i.Size,
	}, nil

}

func (si *image) Pull(ctx context.Context, name string) (*entity.Image, error) {
	reader, err := si.client.ImagePull(context.Background(), name, dockerImage.PullOptions{})
	if err != nil {
		return nil, fmt.Errorf("error pulling image: %v", err)
	}
	defer reader.Close()
	_, err = io.Copy(io.Discard, reader)
	if err != nil {
		return nil, fmt.Errorf("error copying image pull response: %v", err)
	}
	return si.Info(ctx, name)
}

func (si *image) Remove(ctx context.Context, name string) error {
	_, err := si.client.ImageRemove(ctx, name, dockerImage.RemoveOptions{})
	if err != nil {
		return fmt.Errorf("error removing image: %v", err)
	}
	return nil
}
