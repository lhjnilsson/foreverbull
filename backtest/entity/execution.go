package entity

import (
	"errors"
	"time"

	service "github.com/lhjnilsson/foreverbull/service/entity"
)

type ExecutionStatusType string

const (
	ExecutionStatusCreated   ExecutionStatusType = "CREATED"
	ExecutionStatusRunning   ExecutionStatusType = "RUNNING"
	ExecutionStatusCompleted ExecutionStatusType = "COMPLETED"
	ExecutionStatusFailed    ExecutionStatusType = "FAILED"
)

type Execution struct {
	ID         string              `json:"id" mapstructure:"id"`
	Session    string              `json:"session" mapstructure:"session"`
	Calendar   string              `json:"calendar" mapstructure:"calendar"`
	Start      time.Time           `json:"start" mapstructure:"start"`
	End        time.Time           `json:"end" mapstructure:"end"`
	Benchmark  *string             `json:"benchmark" mapstructure:"benchmark"`
	Symbols    []string            `json:"symbols" mapstructure:"symbols"`
	Parameters []service.Parameter `json:"parameters" mapstructure:"parameters"`

	Statuses []ExecutionStatus `json:"statuses"`

	Port     *int   `json:"port" mapstructure:"port"`
	Database string `json:"database" mapstructure:"database"`
}

type ExecutionStatus struct {
	Status     ExecutionStatusType `json:"status"`
	Error      *string             `json:"error"`
	OccurredAt time.Time           `json:"occurred_at"`
}

func (config *Execution) ValidateConfig() error {
	if config.Start.IsZero() {
		return errors.New("start is required")
	}

	if config.End.IsZero() {
		return errors.New("end is required")
	}

	if config.End.Before(config.Start) {
		return errors.New("end must be after start")
	}

	if len(config.Symbols) == 0 {
		return errors.New("symbols is required")
	}

	return nil
}
