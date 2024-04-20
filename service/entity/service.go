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

type Namespace struct {
	Type      string `json:"type" mapstructure:"type"`
	ValueType string `json:"value_type" mapstructure:"value_type"`
}

type ServiceFunctionParameter struct {
	Key     string  `json:"key"`
	Default *string `json:"default"`
	Type    string  `json:"type"`
}

type ServiceFunction struct {
	Name               string                     `json:"name"`
	Parameters         []ServiceFunctionParameter `json:"parameters"`
	ParallelExecution  bool                       `json:"parallel_execution" mapstructure:"parallel_execution"`
	ReturnType         FunctionReturnType         `json:"return_type" mapstructure:"return_type"`
	InputKey           string                     `json:"input_key" mapstructure:"input_key"`
	NamespaceReturnKey *string                    `json:"namespace_return_key" mapstructure:"namespace_return_key"`
}

type Service struct {
	Image     string `json:"image" binding:"required"`
	Algorithm *ServiceAlgorithm
	Statuses  []ServiceStatus `json:"statuses"`
}

type ServiceAlgorithm struct {
	FilePath  string               `json:"file_path" mapstructure:"file_path"`
	Functions []ServiceFunction    `json:"functions"`
	Namespace map[string]Namespace `json:"namespace" mapstructure:"namespace"`
}

type ServiceStatus struct {
	Status     ServiceStatusType `json:"status"`
	Error      *string           `json:"message"`
	OccurredAt time.Time         `json:"occurred_at"`
}
