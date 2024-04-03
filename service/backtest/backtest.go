package backtest

import (
	"context"
	"time"
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
	Amount        int64     `json:"amount" mapstructure:"amount"`
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

type Period struct {
	Timestamp time.Time `json:"timestamp" mapstructure:"timestamp"`

	CashFlow         float64 `json:"cash_flow" mapstructure:"cash_flow"`
	StartingCash     float64 `json:"starting_cash" mapstructure:"starting_cash"`
	PNL              float64 `json:"pnl" mapstructure:"pnl"`
	Returns          float64 `json:"returns" mapstructure:"returns"`
	Cash             float64 `json:"cash" mapstructure:"cash"`
	PortfolioValue   float64 `json:"portfolio_value" mapstructure:"portfolio_value"`
	PortfolioExposed float64 `json:"portfolio_exposed" mapstructure:"portfolio_exposed"`

	Positions []Position `json:"positions" mapstructure:"positions"`
	NewOrders []Order    `json:"new_orders" mapstructure:"new_orders"`
}

type Result struct {
	Periods []struct {
		Timestamp      time.Time `json:"timestamp" mapstructure:"timestamp"`
		PNL            float64   `json:"pnl" mapstructure:"pnl"`
		Returns        float64   `json:"returns" mapstructure:"returns"`
		PortfolioValue float64   `json:"portfolio_value" mapstructure:"portfolio_value"`

		LongsCount       int     `json:"longs_count" mapstructure:"longs_count"`
		ShortsCount      int     `json:"shorts_count" mapstructure:"shorts_count"`
		LongValue        float64 `json:"long_value" mapstructure:"long_value"`
		ShortValue       float64 `json:"short_value" mapstructure:"short_value"`
		StartingExposure float64 `json:"starting_exposure" mapstructure:"starting_exposure"`
		EndingExposure   float64 `json:"ending_exposure" mapstructure:"ending_exposure"`
		LongExposure     float64 `json:"long_exposure" mapstructure:"long_exposure"`
		ShortExposure    float64 `json:"short_exposure" mapstructure:"short_exposure"`

		CapitalUsed   float64 `json:"capital_used" mapstructure:"capital_used"`
		GrossLeverage float64 `json:"gross_leverage" mapstructure:"gross_leverage"`
		NetLeverage   float64 `json:"net_leverage" mapstructure:"net_leverage"`

		StartingValue float64 `json:"starting_value" mapstructure:"starting_value"`
		EndingValue   float64 `json:"ending_value" mapstructure:"ending_value"`
		StartingCash  float64 `json:"starting_cash" mapstructure:"starting_cash"`
		EndingCash    float64 `json:"ending_cash" mapstructure:"ending_cash"`

		MaxDrawdown            float64 `json:"max_drawdown" mapstructure:"max_drawdown"`
		MaxLeverage            float64 `json:"max_leverage" mapstructure:"max_leverage"`
		ExcessReturns          float64 `json:"excess_returns" mapstructure:"excess_returns"`
		TreasuryPeriodReturn   float64 `json:"treasury_period_return" mapstructure:"treasury_period_return"`
		AlgorithmPeriodReturns float64 `json:"algorithm_period_return" mapstructure:"algorithm_period_return"`

		AlgoVolatility *float64 `json:"algo_volatility" mapstructure:"algo_volatility"`
		Sharpe         *float64 `json:"sharpe" mapstructure:"sharpe"`
		Sortino        *float64 `json:"sortino" mapstructure:"sortino"`

		BenchmarkPeriodReturns *float64 `json:"benchmark_period_returns" mapstructure:"benchmark_period_returns"`
		BenchmarkVolatility    *float64 `json:"benchmark_volatility" mapstructure:"benchmark_volatility"`
		Alpha                  *float64 `json:"alpha" mapstructure:"alpha"`
		Beta                   *float64 `json:"beta" mapstructure:"beta"`
	} `json:"periods" mapstructure:"periods"`
}

type Backtest interface {
	Ingest(context.Context, *IngestConfig) error
	UploadIngestion(context.Context, string) error
	DownloadIngestion(context.Context, string) error
	ConfigureExecution(context.Context, *BacktestConfig) error
	RunExecution(context.Context) error
	GetMessage() (*Period, error)
	Continue() error
	GetExecutionResult(execution *Execution) (*Result, error)
	Stop(context.Context) error

	Order(*Order) (*Order, error)
	GetOrder(*Order) (*Order, error)
	CancelOrder(order *Order) error
}
