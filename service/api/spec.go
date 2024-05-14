package api

import "github.com/lhjnilsson/foreverbull/service/entity"

type ServiceURI struct {
	Image string `uri:"image" binding:"required"`
}

type CreateServiceRequest struct {
	Image string `json:"image" binding:"required,gte=3,lte=64"`
}

type ServiceResponse entity.Service

type InstanceURI struct {
}

type InstanceResponse entity.Instance

type ImageURI struct {
	Name string `uri:"name" binding:"required"`
}

type ImageResponse entity.Image

type ConfigureInstanceRequest struct {
	BrokerPort    int                                `json:"broker_port" binding:"required"`
	NamespacePort int                                `json:"namespace_port" binding:"required"`
	DatabaseURL   string                             `json:"database_url" binding:"required"`
	Functions     map[string]entity.InstanceFunction `json:"functions" binding:"required"`
}

type ConfigureInstanceResponse entity.Instance
