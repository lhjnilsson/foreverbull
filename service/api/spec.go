package api

import "github.com/lhjnilsson/foreverbull/service/entity"

type ServiceURI struct {
	Name string `uri:"name" binding:"required"`
}

type CreateServiceRequest struct {
	Name  string `json:"name" binding:"required,gte=3,lte=32"`
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
