package repository_test

import (
	"context"

	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/repository"
	"github.com/lhjnilsson/foreverbull/pkg/pb"
	"github.com/lhjnilsson/foreverbull/pkg/pb/backtest"
)

func (test *ExecutionTest) TestStoreResultKeepsTheReturnBasisAndDividends() {
	ctx := context.Background()
	executions := repository.Execution{Conn: test.conn}
	created, err := executions.Create(ctx, test.execution, nil)
	test.Require().NoError(err)

	benchmarkPrice := 0.05
	period := &backtest.Period{
		Date:                  &pb.Date{Year: 2024, Month: 7, Day: 31},
		AlgorithmPeriodReturn: 0.128124,
		PriceReturn:           0.111963,
		DividendsEarned:       97,
		BenchmarkPriceReturn:  &benchmarkPrice,
	}
	test.Require().NoError(executions.StoreResult(ctx, created.Id, backtest.ReturnBasis_RETURN_BASIS_TOTAL, 2, []*backtest.Period{period}))

	basis, foreignGross, err := executions.GetReturnBasis(ctx, created.Id)
	test.Require().NoError(err)
	test.Equal(backtest.ReturnBasis_RETURN_BASIS_TOTAL, basis)
	test.Equal(int32(2), foreignGross)

	periods, err := executions.GetPeriods(ctx, created.Id)
	test.Require().NoError(err)
	test.Require().Len(periods, 1)
	test.InDelta(0.128124, periods[0].AlgorithmPeriodReturn, 1e-12)
	test.InDelta(0.111963, periods[0].PriceReturn, 1e-12)
	test.InDelta(97, periods[0].DividendsEarned, 1e-12)
	test.Require().NotNil(periods[0].BenchmarkPriceReturn)
	test.InDelta(0.05, *periods[0].BenchmarkPriceReturn, 1e-12)
}

func (test *ExecutionTest) TestARunStoredBeforeDividendsHasNoReturnBasis() {
	ctx := context.Background()
	executions := repository.Execution{Conn: test.conn}
	created, err := executions.Create(ctx, test.execution, nil)
	test.Require().NoError(err)
	// StoreResult always records a return basis. Clear it, and the price-return
	// columns, so the row matches a run stored before dividends were booked as cash.
	test.Require().NoError(executions.StoreResult(ctx, created.Id, backtest.ReturnBasis_RETURN_BASIS_TOTAL, 0, []*backtest.Period{{Date: &pb.Date{Year: 2024, Month: 1, Day: 2}}}))
	_, err = test.conn.Exec(ctx, `UPDATE backtest_execution SET return_basis=NULL, foreign_dividends_gross=NULL WHERE id=$1`, created.Id)
	test.Require().NoError(err)
	_, err = test.conn.Exec(ctx, `UPDATE backtest_execution_period SET price_return=NULL, benchmark_price_return=NULL WHERE backtest_execution=$1`, created.Id)
	test.Require().NoError(err)

	basis, foreignGross, err := executions.GetReturnBasis(ctx, created.Id)
	test.Require().NoError(err)
	test.Equal(backtest.ReturnBasis_RETURN_BASIS_UNSPECIFIED, basis)
	test.Equal(int32(0), foreignGross)
	periods, err := executions.GetPeriods(ctx, created.Id)
	test.Require().NoError(err)
	test.Nil(periods[0].BenchmarkPriceReturn)
}

func (test *ExecutionTest) TestStoreResultStoresNothingWhenAPeriodFails() {
	ctx := context.Background()
	executions := repository.Execution{Conn: test.conn}
	created, err := executions.Create(ctx, test.execution, nil)
	test.Require().NoError(err)
	day := &pb.Date{Year: 2024, Month: 1, Day: 2}
	err = executions.StoreResult(ctx, created.Id, backtest.ReturnBasis_RETURN_BASIS_ADJUSTED_CLOSE, 3, []*backtest.Period{
		{Date: day, PortfolioValue: 10},
		{Date: day, PortfolioValue: 99},
	})
	test.Require().Error(err)
	test.Require().ErrorContains(err, "unique_backtest_execution_period")
	test.Require().ErrorContains(err, "error creating period result")

	var n int
	err = test.conn.QueryRow(ctx, `SELECT COUNT(*) FROM backtest_execution_period WHERE backtest_execution=$1`, created.Id).Scan(&n)
	test.Require().NoError(err)
	test.Zero(n)
	periods, err := executions.GetPeriods(ctx, created.Id)
	test.Require().NoError(err)
	test.Empty(periods)

	var basisName *string
	var foreignGross *int32
	err = test.conn.QueryRow(ctx, `SELECT return_basis, foreign_dividends_gross FROM backtest_execution WHERE id=$1`, created.Id).Scan(&basisName, &foreignGross)
	test.Require().NoError(err)
	test.Nil(basisName)
	test.Nil(foreignGross)
}

func (test *ExecutionTest) TestStoreResultStoresEveryPeriodInOrder() {
	ctx := context.Background()
	executions := repository.Execution{Conn: test.conn}
	created, err := executions.Create(ctx, test.execution, nil)
	test.Require().NoError(err)

	err = executions.StoreResult(ctx, created.Id, backtest.ReturnBasis_RETURN_BASIS_TOTAL, 1, []*backtest.Period{
		{Date: &pb.Date{Year: 2024, Month: 1, Day: 4}, PortfolioValue: 30},
		{Date: &pb.Date{Year: 2024, Month: 1, Day: 2}, PortfolioValue: 10},
		{Date: &pb.Date{Year: 2024, Month: 1, Day: 3}, PortfolioValue: 20},
	})
	test.Require().NoError(err)

	basis, foreignGross, err := executions.GetReturnBasis(ctx, created.Id)
	test.Require().NoError(err)
	test.Equal(backtest.ReturnBasis_RETURN_BASIS_TOTAL, basis)
	test.Equal(int32(1), foreignGross)

	periods, err := executions.GetPeriods(ctx, created.Id)
	test.Require().NoError(err)
	test.Require().Len(periods, 3)
	test.Equal(int32(2), periods[0].GetDate().GetDay())
	test.InDelta(10, periods[0].GetPortfolioValue(), 1e-9)
	test.Equal(int32(3), periods[1].GetDate().GetDay())
	test.InDelta(20, periods[1].GetPortfolioValue(), 1e-9)
	test.Equal(int32(4), periods[2].GetDate().GetDay())
	test.InDelta(30, periods[2].GetPortfolioValue(), 1e-9)
}

func (test *ExecutionTest) TestStoreResultRefusesAnUnspecifiedBasis() {
	ctx := context.Background()
	executions := repository.Execution{Conn: test.conn}
	created, err := executions.Create(ctx, test.execution, nil)
	test.Require().NoError(err)

	err = executions.StoreResult(ctx, created.Id, backtest.ReturnBasis_RETURN_BASIS_UNSPECIFIED, 0, nil)
	test.Require().ErrorContains(err, "unknown return basis")
	err = executions.StoreResult(ctx, "missing", backtest.ReturnBasis_RETURN_BASIS_TOTAL, 0, nil)
	test.Require().ErrorContains(err, "no rows")
}
