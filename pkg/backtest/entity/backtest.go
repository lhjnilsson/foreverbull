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
	Name    string  `json:"name"`
	Service *string `json:"service"`

	Calendar  string    `json:"calendar" mapstructure:"calendar"`
	Start     time.Time `json:"start" mapstructure:"start"`
	End       time.Time `json:"end" mapstructure:"end"`
	Benchmark *string   `json:"benchmark" mapstructure:"benchmark"`
	Symbols   []string  `json:"symbols" mapstructure:"symbols"`

	Statuses []BacktestStatus `json:"statuses"`
	Sessions int              `json:"sessions"`
}

type BacktestStatus struct {
	Status     BacktestStatusType `json:"status"`
	Error      *string            `json:"message"`
	OccurredAt time.Time          `json:"occurred_at"`
}
