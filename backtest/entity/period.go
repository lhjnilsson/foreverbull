package entity

import (
	"time"

	finance "github.com/lhjnilsson/foreverbull/finance/entity"
)

type Period struct {
	ID                     int               `json:"-"`
	Timestamp              time.Time         `json:"timestamp" mapstructure:"timestamp"`
	Portfolio              finance.Portfolio `json:"portfolio" mapstructure:"portfolio"`
	NewOrders              []finance.Order   `json:"new_orders" mapstructure:"new_orders"`
	Symbols                []string          `json:"symbols" mapstructure:"symbols"`
	ShortsCount            int               `json:"shorts_count" mapstructure:"shorts_count"`
	PNL                    int               `json:"pnl" mapstructure:"pnl"`
	LongValue              int               `json:"long_value" mapstructure:"long_value"`
	ShortValue             int               `json:"short_value" mapstructure:"short_value"`
	LongExposure           int               `json:"long_exposure" mapstructure:"long_exposure"`
	StartingExposure       int               `json:"starting_exposure" mapstructure:"starting_exposure"`
	ShortExposure          int               `json:"short_exposure" mapstructure:"short_exposure"`
	CapitalUsed            int               `json:"capital_used" mapstructure:"capital_used"`
	GrossLeverage          int               `json:"gross_leverage" mapstructure:"gross_leverage"`
	NetLeverage            int               `json:"net_leverage" mapstructure:"net_leverage"`
	EndingExposure         int               `json:"ending_exposure" mapstructure:"ending_exposure"`
	StartingValue          int               `json:"starting_value" mapstructure:"starting_value"`
	EndingValue            int               `json:"ending_value" mapstructure:"ending_value"`
	StartingCash           int               `json:"starting_cash" mapstructure:"starting_cash"`
	EndingCash             int               `json:"ending_cash" mapstructure:"ending_cash"`
	Returns                int               `json:"returns" mapstructure:"returns"`
	PortfolioValue         int               `json:"portfolio_value" mapstructure:"portfolio_value"`
	LongsCount             int               `json:"longs_count" mapstructure:"longs_count"`
	AlgoVolatility         int               `json:"algo_volatility" mapstructure:"algo_volatility"`
	Sharpe                 int               `json:"sharpe" mapstructure:"sharpe"`
	Alpha                  int               `json:"alpha" mapstructure:"alpha"`
	Beta                   int               `json:"beta" mapstructure:"beta"`
	Sortino                int               `json:"sortino" mapstructure:"sortino"`
	MaxDrawdown            int               `json:"max_drawdown" mapstructure:"max_drawdown"`
	MaxLeverage            int               `json:"max_leverage" mapstructure:"max_leverage"`
	ExcessReturns          int               `json:"excess_returns" mapstructure:"excess_returns"`
	TreasuryPeriodReturn   int               `json:"treasure_period_return" mapstructure:"treasure_period_return"`
	TradingDays            int               `json:"trading_days" mapstructure:"trading_days"`
	BenchmarkPeriodReturns int               `json:"benchmark_period_returns" mapstructure:"benchmark_period_returns"`
	BenchmarkVolatility    int               `json:"benchmark_volatility" mapstructure:"benchmark_volatility"`
	AlgorithmPeriodReturns int               `json:"algorithm_period_return" mapstructure:"algorithm_period_return"`
}
