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

type Algorithm struct {
	FilePath  string `json:"file_path" mapstructure:"file_path"`
	Functions []struct {
		Name       string `json:"name" mapstructure:"name"`
		Parameters []struct {
			Key     string  `json:"key" mapstructure:"key"`
			Default *string `json:"default" mapstructure:"default"`
			Type    string  `json:"type" mapstructure:"type"`
		} `json:"parameters" mapstructure:"parameters"`
		ParallelExecution *bool `json:"parallel_execution" mapstructure:"parallel_execution"`
	}
}

type ServiceStatus struct {
	Status     ServiceStatusType `json:"status"`
	Error      *string           `json:"message"`
	OccurredAt time.Time         `json:"occurred_at"`
}

type Service struct {
	Image     string     `json:"image" binding:"required"`
	Algorithm *Algorithm `json:"algorithm" binding:"required"`

	Statuses []ServiceStatus `json:"statuses"`
}
