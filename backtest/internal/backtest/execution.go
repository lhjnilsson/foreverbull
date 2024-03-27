package backtest

import (
	"context"
	"fmt"

	"github.com/lhjnilsson/foreverbull/backtest/entity"

	"github.com/lhjnilsson/foreverbull/service/backtest"
	"github.com/lhjnilsson/foreverbull/service/worker"
	"golang.org/x/sync/errgroup"
)

type Execution interface {
	Configure(context.Context, *worker.Configuration, *backtest.BacktestConfig) error
	Run(context.Context, string, chan<- chan entity.ExecutionPeriod)
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
func (b *execution) Run(ctx context.Context, excID string, events chan<- chan entity.ExecutionPeriod) {
	defer close(events)
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
		ch := make(chan entity.ExecutionPeriod)
		events <- ch
		es := entity.ExecutionPeriod{Error: err}
		ch <- es
		return
	}
	g, gctx = errgroup.WithContext(ctx)
	for {
		req, err := b.backtest.GetMessage()
		if err != nil {
			ch := make(chan entity.ExecutionPeriod)
			events <- ch
			es := entity.ExecutionPeriod{Error: fmt.Errorf("error throughout day: %w", err)}
			ch <- es
			return
		}
		if req.Data == nil {
			return // End of backtest
		}
		period := &entity.Period{}
		err = req.DecodeData(period)
		if err != nil {
			ch := make(chan entity.ExecutionPeriod)
			events <- ch
			es := entity.ExecutionPeriod{Error: fmt.Errorf("error decoding period: %w", err)}
			ch <- es
			return
		}
		ch := make(chan entity.ExecutionPeriod)
		events <- ch
		es := entity.ExecutionPeriod{Period: period}
		ch <- es
		<-ch // Wait for close, meaning all data is stored to database
		for _, symbol := range period.Symbols {
			s := symbol
			g.Go(func() error {
				order, err := b.workers.Process(gctx, excID, period.Timestamp, s, &es.Period.Portfolio)
				if err != nil {
					return fmt.Errorf("error processing ohlc: %w", err)
				}
				if order != nil {
					_, err = b.backtest.Order(&backtest.Order{Symbol: *order.Symbol, Amount: *order.Amount})
					if err != nil {
						return fmt.Errorf("error placing order: %w", err)
					}
					return nil
				}
				return err
			})
		}
		if err := g.Wait(); err != nil {
			ch := make(chan entity.ExecutionPeriod)
			events <- ch
			es := entity.ExecutionPeriod{Error: fmt.Errorf("error processing throughout day: %w", err)}
			ch <- es
			return
		}
		if err := b.backtest.Continue(); err != nil {
			ch := make(chan entity.ExecutionPeriod)
			events <- ch
			es := entity.ExecutionPeriod{Error: fmt.Errorf("error continuing execution: %w", err)}
			ch <- es
			return
		}
	}
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
