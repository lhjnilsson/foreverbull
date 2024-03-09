package api

import "context"

type Client interface {
	ListServices(ctx context.Context) (*[]ServiceResponse, error)
	GetService(ctx context.Context, name string) (*ServiceResponse, error)

	ListInstances(ctx context.Context, serviceName string) (*[]InstanceResponse, error)
	GetInstance(ctx context.Context, serviceName string, InstanceID string) (*InstanceResponse, error)

	GetImage(ctx context.Context, image string) (*ImageResponse, error)
	DownloadImage(ctx context.Context, image string) (*ImageResponse, error)
}
