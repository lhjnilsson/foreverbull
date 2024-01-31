package engine

import (
	"context"
	"time"

	"github.com/lhjnilsson/foreverbull/service/message"
)

type IngestConfig struct {
	Calendar string    `json:"calendar" mapstructure:"calendar"`
	Start    time.Time `json:"start" mapstructure:"start"`
	End      time.Time `json:"end" mapstructure:"end"`
	Symbols  []string  `json:"symbols" mapstructure:"symbols"`
	Database string    `json:"database" mapstructure:"database"`
}

type BacktestConfig struct {
	Calendar  *string    `json:"calendar" mapstructure:"calendar"`
	Start     *time.Time `json:"start" mapstructure:"start"`
	End       *time.Time `json:"end" mapstructure:"end"`
	Timezone  *string    `json:"timezone" mapstructure:"timezone"`
	Benchmark *string    `json:"benchmark" mapstructure:"benchmark"`
	Symbols   *[]string  `json:"symbols" mapstructure:"symbols"`
}

type Execution struct {
	Execution string `json:"execution" mapstructure:"execution"`
}

/*
Engine
Interface for the backtest- engine
*/
type Engine interface {
	Ingest(context.Context, *IngestConfig) error
	UploadIngestion(context.Context, string) error
	DownloadIngestion(context.Context, string) error
	GetBroker() Broker
	ConfigureExecution(context.Context, *BacktestConfig) error
	RunExecution(context.Context) error
	GetMessage() (*message.Response, error)
	Continue() error
	GetExecutionResult(execution *Execution) (*message.Response, error)
	Stop(context.Context) error
}
