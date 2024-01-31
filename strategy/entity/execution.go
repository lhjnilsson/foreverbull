package entity

import "time"

type Execution struct {
	ID                *string    `json:"id" mapstructure:"id"`
	Strategy          *string    `json:"strategy" mapstructure:"strategy"`
	StartedAt         *time.Time `json:"started_at" mapstructure:"started_at"`
	IngestedAt        *time.Time `json:"ingested_at" mapstructure:"ingested_at"`
	BacktestAt        *time.Time `json:"backtest_at" mapstructure:"backtest_at"`
	BacktestSessionID *string    `json:"backtest_session_id" mapstructure:"backtest_session_id"`
}
