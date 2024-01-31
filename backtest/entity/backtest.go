package entity

import (
	"time"
)

type BacktestStatusType string

const (
	BacktestStatusCreated   BacktestStatusType = "CREATED"
	BacktestStatusUpdated   BacktestStatusType = "UPDATED"
	BacktestStatusIngesting BacktestStatusType = "INGESTING"
	BacktestStatusReady     BacktestStatusType = "READY"
	BacktestStatusError     BacktestStatusType = "ERROR"
)

type Backtest struct {
	Name            string           `json:"name"`
	BacktestService string           `json:"backtest_service" required:"true"`
	WorkerService   *string          `json:"worker_service"`
	Calendar        string           `json:"calendar" mapstructure:"calendar" required:"true"`
	Start           time.Time        `json:"start" mapstructure:"start" required:"true"`
	End             time.Time        `json:"end" mapstructure:"end" required:"true"`
	Benchmark       *string          `json:"benchmark" mapstructure:"benchmark" required:"true"`
	Symbols         []string         `json:"symbols" mapstructure:"symbols" required:"true"`
	Statuses        []BacktestStatus `json:"statuses"`
	Sessions        int              `json:"sessions"`
}

type BacktestStatus struct {
	Status     BacktestStatusType `json:"status"`
	Error      *string            `json:"message"`
	OccurredAt time.Time          `json:"occurred_at"`
}
