package backtest

import (
	"context"
	"fmt"

	"github.com/lhjnilsson/foreverbull/backtest/entity"
	finance "github.com/lhjnilsson/foreverbull/finance/entity"
	"github.com/lhjnilsson/foreverbull/service/backtest"
	service "github.com/lhjnilsson/foreverbull/service/entity"
	"github.com/lhjnilsson/foreverbull/service/worker"
	"github.com/shopspring/decimal"
	"golang.org/x/sync/errgroup"
)

type Execution interface {
	Configure(context.Context, *service.Instance, *backtest.BacktestConfig) error
	Run(context.Context, *entity.Execution) error
	StoreDataFrameAndGetPeriods(context.Context, string) (*[]entity.Period, error)
	Stop(context.Context) error
}

func NewExecution(b backtest.Backtest, w worker.Pool) Execution {
	return &execution{
		backtest: b,
		workers:  w,
	}
}

type execution struct {
	backtest backtest.Backtest `json:"-"`
	workers  worker.Pool       `json:"-"`
}

/*
Configure
*/
func (b *execution) Configure(ctx context.Context, instance *service.Instance, backtestCfg *backtest.BacktestConfig) error {
	g, gctx := errgroup.WithContext(ctx)
	if instance != nil {
		g.Go(func() error { return b.workers.ConfigureExecution(gctx, instance) })
	}
	g.Go(func() error { return b.backtest.ConfigureExecution(gctx, backtestCfg) })
	return g.Wait()
}

/*
Run
Runs configured workers and backtest until completed
*/
func (b *execution) Run(ctx context.Context, execution *entity.Execution) error {
	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		err := b.backtest.RunExecution(gctx)
		if err != nil {
			return fmt.Errorf("error running Execution engine: %w", err)
		}
		return nil
	})
	g.Go(func() error {
		err := b.workers.RunExecution(gctx)
		if err != nil {
			return fmt.Errorf("error running Execution workers: %w", err)
		}
		return nil
	})
	if err := g.Wait(); err != nil {
		return fmt.Errorf("error running Execution: %w", err)
	}
	for {
		period, err := b.backtest.GetMessage()
		if err != nil {
			return fmt.Errorf("error getting message: %w", err)
		}
		if period == nil {
			return nil
		}
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

		orders, err := b.workers.Process(gctx, period.Timestamp, execution.Symbols, &portfolio)
		if err != nil {
			return fmt.Errorf("error processing ohlc: %w", err)
		}
		for _, order := range *orders {
			_, err = b.backtest.Order(&backtest.Order{Symbol: order.Symbol, Amount: int(order.Amount.IntPart())})
			if err != nil {
				return fmt.Errorf("error placing order: %w", err)
			}
		}
		if err := b.backtest.Continue(); err != nil {
			return fmt.Errorf("error continuing backtest: %w", err)
		}
	}
}

type Result struct {
	Periods []entity.Period `json:"periods"`
}

func (b *execution) StoreDataFrameAndGetPeriods(ctx context.Context, excID string) (*[]entity.Period, error) {
	result, err := b.backtest.GetExecutionResult(&backtest.Execution{Execution: excID})
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
	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		if err := b.backtest.Stop(gctx); err != nil {
			return fmt.Errorf("error stopping Execution engine: %w", err)
		}
		return nil
	})
	g.Go(func() error {
		if err := b.workers.Stop(gctx); err != nil {
			return fmt.Errorf("error stopping workers: %w", err)
		}
		return nil
	})
	err := g.Wait()
	if err != nil {
		return fmt.Errorf("error stopping Execution: %w", err)
	}
	return nil
}
