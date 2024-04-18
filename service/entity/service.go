package entity

import (
	"time"
)

type ServiceStatusType string

const (
	ServiceStatusCreated   ServiceStatusType = "CREATED"
	ServiceStatusInterview ServiceStatusType = "INTERVIEW"
	ServiceStatusReady     ServiceStatusType = "READY"
	ServiceStatusError     ServiceStatusType = "ERROR"
)

type FunctionReturnType string

const (
	OrderReturnType          FunctionReturnType = "ORDER"
	ListOfOrdersReturnType   FunctionReturnType = "LIST_OF_ORDERS"
	NamespaceValueReturnType FunctionReturnType = "NAMESPACE_VALUE"
)

type Namespace struct{}

type Service struct {
	Image     string `json:"image" binding:"required"`
	Algorithm *ServiceAlgorithm
	Statuses  []ServiceStatus `json:"statuses"`
}

type ServiceAlgorithm struct {
	FilePath  string `json:"file_path"`
	Functions []struct {
		Name       string `json:"name"`
		Parameters []struct {
			Key     string `json:"key"`
			Default string `json:"default"`
			Type    string `json:"type"`
		} `json:"parameters"`
		ParallelExecution bool               `json:"parallel_execution"`
		ReturnType        FunctionReturnType `json:"return_type"`
	} `json:"functions"`
	Namespace Namespace `json:"namespace"`
}

type ServiceStatus struct {
	Status     ServiceStatusType `json:"status"`
	Error      *string           `json:"message"`
	OccurredAt time.Time         `json:"occurred_at"`
}
