package backtest

import (
	"context"
	"fmt"

	"github.com/lhjnilsson/foreverbull/pkg/backtest/engine"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/entity"
	finance "github.com/lhjnilsson/foreverbull/pkg/finance/entity"
	"github.com/lhjnilsson/foreverbull/pkg/service/worker"
	"github.com/shopspring/decimal"
)

type Execution interface {
	Configure(context.Context, *entity.Execution) error
	Run(context.Context, *entity.Execution) error
	CurrentPeriod() *engine.Period
	StoreDataFrameAndGetPeriods(context.Context, string) (*[]entity.Period, error)
	Stop(context.Context) error
}

func NewExecution(e engine.Engine, w worker.Pool) Execution {
	return &execution{
		engine:  e,
		workers: w,
	}
}

type execution struct {
	engine  engine.Engine `json:"-"`
	workers worker.Pool   `json:"-"`

	currentPeriod *engine.Period `json:"-"`
}

/*
Configure
*/
func (b *execution) Configure(ctx context.Context, execution *entity.Execution) error {
	return b.engine.ConfigureExecution(ctx, execution)
}

/*
Run
Runs configured workers and backtest until completed
*/
func (b *execution) Run(ctx context.Context, execution *entity.Execution) error {
	err := b.engine.RunExecution(ctx)
	if err != nil {
		return fmt.Errorf("error running Execution engine: %w", err)
	}
	defer func() {
		b.currentPeriod = nil
	}()
	for {
		period, err := b.engine.GetMessage()
		if err != nil {
			return fmt.Errorf("error getting message: %w", err)
		}
		if period == nil {
			return nil
		}
		b.currentPeriod = period

		positions := make([]finance.Position, 0)
		for _, p := range period.Positions {
			positions = append(positions, finance.Position{
				Symbol:    p.Symbol,
				Amount:    decimal.NewFromInt(p.Amount),
				CostBasis: decimal.NewFromFloat(p.CostBasis),
			})
		}

		portfolio := finance.Portfolio{
			Cash:      decimal.NewFromFloat(period.Cash),
			Value:     decimal.NewFromFloat(period.PortfolioValue),
			Positions: positions,
		}

		orders, err := b.workers.Process(ctx, period.Timestamp, execution.Symbols, &portfolio)
		if err != nil {
			return fmt.Errorf("error processing ohlc: %w", err)
		}
		for _, order := range *orders {
			_, err = b.engine.Order(&engine.Order{Symbol: order.Symbol, Amount: int(order.Amount.IntPart())})
			if err != nil {
				return fmt.Errorf("error placing order: %w", err)
			}
		}
		if err := b.engine.Continue(); err != nil {
			return fmt.Errorf("error continuing backtest: %w", err)
		}
	}
}

func (b *execution) CurrentPeriod() *engine.Period {
	return b.currentPeriod
}

func (b *execution) StoreDataFrameAndGetPeriods(ctx context.Context, excID string) (*[]entity.Period, error) {
	result, err := b.engine.GetExecutionResult(excID)
	if err != nil {
		return nil, fmt.Errorf("error getting data for result: %v", err)
	}
	var periods []entity.Period
	for _, p := range result.Periods {
		periods = append(periods, entity.Period{
			Timestamp:              p.Timestamp,
			PNL:                    p.PNL,
			Returns:                p.Returns,
			PortfolioValue:         p.PortfolioValue,
			LongsCount:             p.LongsCount,
			ShortsCount:            p.ShortsCount,
			LongValue:              p.LongValue,
			ShortValue:             p.ShortValue,
			StartingExposure:       p.StartingExposure,
			EndingExposure:         p.EndingExposure,
			LongExposure:           p.LongExposure,
			ShortExposure:          p.ShortExposure,
			CapitalUsed:            p.CapitalUsed,
			GrossLeverage:          p.GrossLeverage,
			NetLeverage:            p.NetLeverage,
			StartingValue:          p.StartingValue,
			EndingValue:            p.EndingValue,
			StartingCash:           p.StartingCash,
			EndingCash:             p.EndingCash,
			MaxDrawdown:            p.MaxDrawdown,
			MaxLeverage:            p.MaxLeverage,
			ExcessReturns:          p.ExcessReturns,
			TreasuryPeriodReturn:   p.TreasuryPeriodReturn,
			AlgorithmPeriodReturns: p.AlgorithmPeriodReturns,
			AlgoVolatility:         p.AlgoVolatility,
			Sharpe:                 p.Sharpe,
			Sortino:                p.Sortino,
			BenchmarkPeriodReturns: p.BenchmarkPeriodReturns,
			BenchmarkVolatility:    p.BenchmarkVolatility,
			Alpha:                  p.Alpha,
			Beta:                   p.Beta,
		})
	}
	return &periods, nil
}

/*
Stop
Halts all workers and backtest
*/
func (b *execution) Stop(ctx context.Context) error {
	if err := b.engine.Stop(ctx); err != nil {
		return fmt.Errorf("error stopping Execution engine: %w", err)
	}
	return nil
}
