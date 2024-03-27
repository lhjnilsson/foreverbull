package entity

import "time"

type ExecutionStatusType string

const (
	ExecutionStatusCreated   ExecutionStatusType = "CREATED"
	ExecutionStatusStarted   ExecutionStatusType = "STARTED"
	ExecutionStatusRunning   ExecutionStatusType = "RUNNING"
	ExecutionStatusCompleted ExecutionStatusType = "COMPLETED"
	ExecutionStatusFailed    ExecutionStatusType = "FAILED"
)

type Execution struct {
	ID       string    `json:"id" mapstructure:"id"`
	Strategy string    `json:"strategy" mapstructure:"strategy"`
	Start    time.Time `json:"start" mapstructure:"start"`
	End      time.Time `json:"end" mapstructure:"end"`
	Service  string    `json:"service" mapstructure:"service"`

	Statuses []ExecutionStatus `json:"statuses"`
}

type ExecutionStatus struct {
	Status     ExecutionStatusType `json:"status"`
	Error      *string             `json:"message"`
	OccurredAt time.Time           `json:"occurred_at"`
}
