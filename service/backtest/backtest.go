package backtest

import (
	"context"
	"time"

	"github.com/lhjnilsson/foreverbull/service/message"
)

type OrderStatus int

const (
	OPEN      OrderStatus = iota
	FILLED    OrderStatus = iota
	CANCELLED OrderStatus = iota
	REJECTED  OrderStatus = iota
	HELD      OrderStatus = iota
)

type Order struct {
	Symbol string      `json:"symbol" mapstructure:"symbol"`
	Amount int         `json:"amount" mapstructure:"amount"`
	Status OrderStatus `json:"status" mapstructure:"status"`
}

type Position struct {
	Symbol        string    `json:"symbol" mapstructure:"symbol"`
	Amount        int       `json:"amount" mapstructure:"amount"`
	CostBasis     float64   `json:"cost_basis" mapstructure:"cost_basis"`
	LastSalePrice float64   `json:"last_price" mapstructure:"last_sale_price"`
	LastSaleDate  time.Time `json:"last_sale_date" mapstructure:"last_sale_date"`
}

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

type Backtest interface {
	Ingest(context.Context, *IngestConfig) error
	UploadIngestion(context.Context, string) error
	DownloadIngestion(context.Context, string) error
	ConfigureExecution(context.Context, *BacktestConfig) error
	RunExecution(context.Context) error
	GetMessage() (*message.Response, error)
	Continue() error
	GetExecutionResult(execution *Execution) (*message.Response, error)
	Stop(context.Context) error

	Order(*Order) (*Order, error)
	GetOrder(*Order) (*Order, error)
	CancelOrder(order *Order) error
}
