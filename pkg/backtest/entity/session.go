package entity

import (
	"time"
)

type SessionStatusType string

const (
	SessionStatusCreated   SessionStatusType = "CREATED"
	SessionStatusRunning   SessionStatusType = "RUNNING"
	SessionStatusCompleted SessionStatusType = "COMPLETED"
	SessionStatusFailed    SessionStatusType = "FAILED"
)

type Session struct {
	ID       string          `json:"id" mapstructure:"id"`
	Manual   bool            `json:"manual" mapstructure:"manual"`
	Backtest string          `json:"backtest" mapstructure:"backtest"`
	Statuses []SessionStatus `json:"statuses"`

	Executions int `json:"executions" mapstructure:"executions"`

	Port *int `json:"port" mapstructure:"port"`
}

type SessionStatus struct {
	Status     SessionStatusType `json:"status"`
	Error      *string           `json:"error"`
	OccurredAt time.Time         `json:"occurred_at"`
}
