package backtest

import (
	"context"
	"fmt"

	"github.com/lhjnilsson/foreverbull/backtest/entity"
	finance "github.com/lhjnilsson/foreverbull/finance/entity"
	"github.com/lhjnilsson/foreverbull/service/backtest"
	"github.com/lhjnilsson/foreverbull/service/worker"
	"github.com/shopspring/decimal"
	"golang.org/x/sync/errgroup"
)

type Execution interface {
	Configure(context.Context, *worker.Configuration, *backtest.BacktestConfig) error
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
func (b *execution) Configure(ctx context.Context, workerCfg *worker.Configuration, backtestCfg *backtest.BacktestConfig) error {
	g, gctx := errgroup.WithContext(ctx)
	if workerCfg != nil {
		g.Go(func() error { return b.workers.ConfigureExecution(gctx, workerCfg) })
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
	g, gctx = errgroup.WithContext(ctx)
	for {
		req, err := b.backtest.GetMessage()
		if err != nil {
			return fmt.Errorf("error getting message: %w", err)
		}
		if req.Data == nil {
			return nil
		}
		period := &entity.RunningPeriod{}
		err = req.DecodeData(period)
		if err != nil {
			return fmt.Errorf("error decoding data: %w", err)
		}
		portfolio := finance.Portfolio{
			Cash:           decimal.NewFromFloat(period.Cash),
			PortfolioValue: decimal.NewFromFloat(period.PortfolioValue),
		}

		for _, symbol := range execution.Symbols {
			s := symbol
			g.Go(func() error {
				order, err := b.workers.Process(gctx, execution.ID, period.Timestamp, s, &portfolio)
				if err != nil {
					return fmt.Errorf("error processing ohlc: %w", err)
				}
				if order != nil {
					_, err = b.backtest.Order(&backtest.Order{Symbol: order.Symbol, Amount: int(order.Amount.IntPart())})
					if err != nil {
						return fmt.Errorf("error placing order: %w", err)
					}
					return nil
				}
				return err
			})
		}
		if err := g.Wait(); err != nil {
			return fmt.Errorf("error processing ohlc: %w", err)
		}
		if err := b.backtest.Continue(); err != nil {
			return fmt.Errorf("error continuing backtest: %w", err)
		}
	}
	return nil
}

type Result struct {
	Periods []entity.Period `json:"periods"`
}

func (b *execution) StoreDataFrameAndGetPeriods(ctx context.Context, excID string) (*[]entity.Period, error) {
	result := Result{}
	req, err := b.backtest.GetExecutionResult(&backtest.Execution{Execution: excID})
	if err != nil {
		return nil, fmt.Errorf("error getting data for result: %v", err)
	}
	err = req.DecodeData(&result)
	if err != nil {
		return nil, fmt.Errorf("error decoding data from result: %v", err)
	}
	return &result.Periods, nil
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
