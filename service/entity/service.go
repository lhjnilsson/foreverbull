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

type Parameter struct {
	Key     string `json:"key"`
	Type    string `json:"type"`
	Value   string `json:"value"`
	Default string `json:"default"`
}

type Service struct {
	Image      string       `json:"image" binding:"required"`
	Parameters *[]Parameter `json:"parameters" mapstructure:"parameters"`

	Statuses []ServiceStatus `json:"statuses"`
}

type ServiceStatus struct {
	Status     ServiceStatusType `json:"status"`
	Error      *string           `json:"message"`
	OccurredAt time.Time         `json:"occurred_at"`
}
