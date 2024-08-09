package entity

import (
	"time"
)

type Period struct {
	Timestamp      time.Time `json:"timestamp" mapstructure:"timestamp"`
	PNL            float64   `json:"pnl" mapstructure:"pnl"`
	Returns        float64   `json:"returns" mapstructure:"returns"`
	PortfolioValue float64   `json:"portfolio_value" mapstructure:"portfolio_value"`

	LongsCount       int32   `json:"longs_count" mapstructure:"longs_count"`
	ShortsCount      int32   `json:"shorts_count" mapstructure:"shorts_count"`
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

	MaxDrawdown           float64 `json:"max_drawdown" mapstructure:"max_drawdown"`
	MaxLeverage           float64 `json:"max_leverage" mapstructure:"max_leverage"`
	ExcessReturns         float64 `json:"excess_returns" mapstructure:"excess_returns"`
	TreasuryPeriodReturn  float64 `json:"treasury_period_return" mapstructure:"treasury_period_return"`
	AlgorithmPeriodReturn float64 `json:"algorithm_period_return" mapstructure:"algorithm_period_return"`

	AlgoVolatility *float64 `json:"algo_volatility" mapstructure:"algo_volatility"`
	Sharpe         *float64 `json:"sharpe" mapstructure:"sharpe"`
	Sortino        *float64 `json:"sortino" mapstructure:"sortino"`

	BenchmarkPeriodReturn *float64 `json:"benchmark_period_return" mapstructure:"benchmark_period_return"`
	BenchmarkVolatility   *float64 `json:"benchmark_volatility" mapstructure:"benchmark_volatility"`
	Alpha                 *float64 `json:"alpha" mapstructure:"alpha"`
	Beta                  *float64 `json:"beta" mapstructure:"beta"`
}
