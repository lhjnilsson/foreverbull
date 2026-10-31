package command_test

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/engine"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/repository"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/stream/command"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/stream/dependency"
	"github.com/lhjnilsson/foreverbull/pkg/pb"
	"github.com/lhjnilsson/foreverbull/pkg/pb/backtest"
	financeServicePB "github.com/lhjnilsson/foreverbull/pkg/pb/finance/service"
	"github.com/stretchr/testify/mock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// The backtest the copy in the brief names: OEF as the benchmark, 2024 Q1.
const quarterBacktest = "oef-2024q1"

var (
	quarterStart = time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	quarterEnd   = time.Date(2024, 3, 31, 0, 0, 0, 0, time.UTC)
	// afterEaster runs past Good Friday, so a bar on 2024-04-01 puts
	// 2024-03-29 inside the symbol's span.
	afterEaster = time.Date(2024, 4, 5, 0, 0, 0, 0, time.UTC)
)

// xnysSessions is what Zipline's XNYS calendar answers from 2024-01-01 to
// end: weekdays without New Year's Day, MLK Day, Presidents Day and Good
// Friday. The zipline tests check the real calendar leaves Good Friday out.
func xnysSessions(end time.Time) []time.Time {
	holidays := map[string]bool{"2024-01-01": true, "2024-01-15": true, "2024-02-19": true, "2024-03-29": true}
	sessions := make([]time.Time, 0, 70)
	for day := quarterStart; !day.After(end); day = day.AddDate(0, 0, 1) {
		if day.Weekday() == time.Saturday || day.Weekday() == time.Sunday || holidays[day.Format(time.DateOnly)] {
			continue
		}
		sessions = append(sessions, day)
	}
	return sessions
}

// resetBars gives the suite an empty finance ohlc table. Recreate only
// rebuilds the backtest tables, and the finance module owns this one.
func (test *BacktestCommandTest) resetBars() {
	_, err := test.db.Exec(context.Background(), `
		DROP TABLE IF EXISTS ohlc;
		DROP TABLE IF EXISTS asset;
		CREATE TABLE asset (symbol text primary key, name text);
		CREATE TABLE ohlc (
			symbol TEXT references asset(symbol),
			open numeric NOT NULL, high numeric NOT NULL, low numeric NOT NULL, close numeric NOT NULL,
			volume bigint NOT NULL,
			time timestamp,
			provider text NOT NULL,
			UNIQUE (provider, symbol, time));`)
	test.Require().NoError(err)
}

// storeBars stores a yahoo bar for symbol on every session from first to
// last except skip. Yahoo stamps the 09:30 ET open, 14:30 UTC.
func (test *BacktestCommandTest) storeBars(symbol, first, last string, skip ...string) {
	ctx := context.Background()
	_, err := test.db.Exec(ctx, `INSERT INTO asset (symbol, name) VALUES ($1, $1) ON CONFLICT DO NOTHING`, symbol)
	test.Require().NoError(err)
	skipped := make(map[string]bool, len(skip))
	for _, day := range skip {
		skipped[day] = true
	}
	for _, session := range xnysSessions(afterEaster) {
		day := session.Format(time.DateOnly)
		if day < first || day > last || skipped[day] {
			continue
		}
		_, err = test.db.Exec(ctx, `
			INSERT INTO ohlc (symbol, open, high, low, close, volume, time, provider)
			VALUES ($1, 10, 11, 9, 10, 100, $2, 'yahoo')`, symbol, session.Add(14*time.Hour+30*time.Minute))
		test.Require().NoError(err)
	}
}

// createQuarter stores the backtest that CREATING would have stored, for
// 2024 Q1.
func (test *BacktestCommandTest) createQuarter(symbols ...string) {
	test.createBacktest(quarterEnd, symbols...)
}

func (test *BacktestCommandTest) createBacktest(end time.Time, symbols ...string) {
	assets := make([]*backtest.Backtest_Period_Asset, 0, len(symbols))
	for _, symbol := range symbols {
		assets = append(assets, &backtest.Backtest_Period_Asset{Symbol: symbol, Enabled: true})
	}
	backtests := repository.Backtest{Conn: test.db}
	_, err := backtests.Create(context.TODO(), &backtest.Backtest{
		Name:      quarterBacktest,
		Benchmark: "OEF",
		Periods: []*backtest.Backtest_Period{{
			StartDate: pb.GoTimeToDate(quarterStart),
			EndDate:   pb.GoTimeToDate(end),
		}},
	})
	test.Require().NoError(err)
	err = backtests.UpdatePeriod(context.TODO(), quarterBacktest, &backtest.Backtest_Period{
		StartDate: pb.GoTimeToDate(quarterStart),
		EndDate:   pb.GoTimeToDate(end),
		Assets:    assets,
	})
	test.Require().NoError(err)
	test.storeBars("OEF", "2024-01-01", end.Format(time.DateOnly))
}

// downloadQuarter runs DOWNLOADING for 2024 Q1 with a finance stream that
// stored every symbol OK, and an engine whose calendar is XNYS.
func (test *BacktestCommandTest) downloadQuarter() (*engine.MockEngine, error) {
	return test.download(quarterEnd)
}

func (test *BacktestCommandTest) download(end time.Time, answers ...*financeServicePB.DownloadHistoricalDataResponse) (*engine.MockEngine, error) {
	clientStream := new(financeServicePB.MockMarketdata_DownloadHistoricalDataClient[interface{}])
	for _, answer := range answers {
		clientStream.On("Recv", mock.Anything).Return(answer, nil).Once()
	}
	clientStream.On("Recv", mock.Anything).Return(nil, io.EOF)
	test.mdclient.On("DownloadHistoricalData", mock.Anything, mock.Anything).Return(clientStream, nil)

	ziplineEngine := new(engine.MockEngine)
	ziplineEngine.On("Sessions", mock.Anything, quarterStart, end).Return(xnysSessions(end), nil)
	msg := test.downloadMessage(quarterBacktest)
	msg.On("Call", mock.Anything, dependency.GetEngineKey).Return(ziplineEngine, nil)
	return ziplineEngine, command.DownloadFinancialData(context.TODO(), msg)
}

// requireNothingSaved checks the failed create left no backtest, period or
// asset behind, so the same create can run again.
func (test *BacktestCommandTest) requireNothingSaved() {
	backtests := repository.Backtest{Conn: test.db}
	_, err := backtests.Get(context.TODO(), quarterBacktest)
	test.Require().ErrorIs(err, pgx.ErrNoRows)
	var periods, assets int
	test.Require().NoError(test.db.QueryRow(context.TODO(),
		`SELECT count(*) FROM backtest_period WHERE backtest_name=$1`, quarterBacktest).Scan(&periods))
	test.Require().NoError(test.db.QueryRow(context.TODO(),
		`SELECT count(*) FROM backtest_period_asset WHERE backtest_name=$1`, quarterBacktest).Scan(&assets))
	test.Zero(periods)
	test.Zero(assets)
}

func (test *BacktestCommandTest) requireKept() {
	backtests := repository.Backtest{Conn: test.db}
	_, err := backtests.Get(context.TODO(), quarterBacktest)
	test.Require().NoError(err)
}

// Quant check 1: one missing session fails create, nothing is saved, and the
// message names the symbol and the day.
func (test *BacktestCommandTest) TestMissingSessionOneSymbolOneDay() {
	test.createQuarter("XYZ")
	test.storeBars("XYZ", "2024-01-01", "2024-03-31", "2024-02-14")

	_, err := test.downloadQuarter()
	test.Require().EqualError(err, "Couldn't load data for backtest 'oef-2024q1': yahoo is missing prices for XYZ on "+
		"2024-02-14, a trading day. The backtest wasn't saved. Try another provider with --provider, or change the "+
		"backtest dates, then run `fbull backtest create` again.")
	test.requireNothingSaved()
}

func (test *BacktestCommandTest) TestMissingSessionOneSymbolSeveralDays() {
	test.createQuarter("XYZ")
	test.storeBars("XYZ", "2024-01-01", "2024-03-31", "2024-02-14", "2024-02-20", "2024-02-21")

	_, err := test.downloadQuarter()
	test.Require().EqualError(err, "Couldn't load data for backtest 'oef-2024q1': yahoo is missing prices for XYZ on "+
		"3 trading days: 2024-02-14, 2024-02-20 to 2024-02-21. The backtest wasn't saved. Try another provider with "+
		"--provider, or change the backtest dates, then run `fbull backtest create` again.")
	test.requireNothingSaved()
}

// Quant check 2: two symbols with 1 and 3 missing sessions fail with one
// error that names both and all four days, in the order they were listed.
func (test *BacktestCommandTest) TestMissingSessionsTwoSymbols() {
	test.createQuarter("AAA", "BBB")
	test.storeBars("AAA", "2024-01-01", "2024-03-31", "2024-02-14")
	test.storeBars("BBB", "2024-01-01", "2024-03-31", "2024-02-20", "2024-02-21", "2024-03-05")

	_, err := test.downloadQuarter()
	test.Require().EqualError(err, "Couldn't load data for backtest 'oef-2024q1': yahoo is missing prices on trading "+
		"days for 2 symbols. The backtest wasn't saved.\n"+
		"  AAA: 2024-02-14\n"+
		"  BBB: 2024-02-20 to 2024-02-21, 2024-03-05\n"+
		"Try another provider with --provider, or change the backtest dates, then run `fbull backtest create` again.")
	test.requireNothingSaved()
}

// A run of missing sessions is one range even across a holiday: Friday
// 2024-02-16 and Tuesday 2024-02-20 are sessions in a row (Presidents Day).
func (test *BacktestCommandTest) TestMissingSessionsRangeSpansAHoliday() {
	test.createQuarter("XYZ")
	test.storeBars("XYZ", "2024-01-01", "2024-03-31", "2024-02-16", "2024-02-20")

	_, err := test.downloadQuarter()
	test.Require().ErrorContains(err, "for XYZ on 2 trading days: 2024-02-16 to 2024-02-20. ")
}

// A row stored as 0 before the broker refused non-positive prices is not a
// price, so its day is missing, as the ingester reads it.
func (test *BacktestCommandTest) TestMissingSessionCountsAZeroPricedRow() {
	test.createQuarter("XYZ")
	test.storeBars("XYZ", "2024-01-01", "2024-03-31")
	_, err := test.db.Exec(context.TODO(), `UPDATE ohlc SET open = 0, high = 0, low = 0, close = 0
		WHERE symbol = 'XYZ' AND time::date = '2024-02-14'`)
	test.Require().NoError(err)

	_, err = test.downloadQuarter()
	test.Require().ErrorContains(err, "yahoo is missing prices for XYZ on 2024-02-14, a trading day.")
	test.requireNothingSaved()
}

// Quant check 3: a symbol that lists after the period starts is not missing
// the sessions before its first bar.
func (test *BacktestCommandTest) TestMissingSessionsIgnoreDaysBeforeTheFirstBar() {
	test.createQuarter("XYZ")
	test.storeBars("XYZ", "2024-02-01", "2024-03-31")

	ziplineEngine, err := test.downloadQuarter()
	test.Require().NoError(err)
	ziplineEngine.AssertExpectations(test.T())
	test.requireKept()
}

// Quant check 4: a symbol whose last bar is before the period end (a
// delisting) is not missing the sessions after it. universe.md §8 still
// closes the position at that last price: the golden runs in
// client/zipline/tests/test_delisted_position.py ingest through the same
// refusal and check it.
func (test *BacktestCommandTest) TestMissingSessionsIgnoreDaysAfterTheLastBar() {
	test.createQuarter("XYZ")
	test.storeBars("XYZ", "2024-01-01", "2024-03-01")

	_, err := test.downloadQuarter()
	test.Require().NoError(err)
	test.requireKept()
}

// Quant check 5: Good Friday 2024-03-29 is not an XNYS session, so having
// no bar that day, between bars on 2024-03-28 and 2024-04-01, is not a
// missing price.
func (test *BacktestCommandTest) TestMissingSessionsSkipGoodFriday() {
	test.createBacktest(afterEaster, "XYZ")
	test.storeBars("XYZ", "2024-01-01", "2024-04-05")
	test.NotContains(xnysSessions(afterEaster), time.Date(2024, 3, 29, 0, 0, 0, 0, time.UTC))

	ziplineEngine, err := test.download(afterEaster)
	test.Require().NoError(err)
	ziplineEngine.AssertExpectations(test.T())
	test.requireKept()
}

// The symbols named are the request symbols. BRK.B is stored as BRKB, and
// the message still says BRK.B.
func (test *BacktestCommandTest) TestMissingSessionNamesTheRequestSymbol() {
	test.createQuarter("BRK.B")
	test.storeBars("BRKB", "2024-01-01", "2024-03-31", "2024-03-05")

	_, err := test.downloadQuarter()
	test.Require().ErrorContains(err, "yahoo is missing prices for BRK.B on 2024-03-05, a trading day.")
}

// With no stored bars there is nothing to check, and Zipline is not asked.
func (test *BacktestCommandTest) TestMissingSessionsNoBarsSkipsTheCalendar() {
	test.createQuarter("XYZ")
	_, err := test.db.Exec(context.TODO(), `DELETE FROM ohlc`)
	test.Require().NoError(err)

	ziplineEngine, err := test.downloadQuarter()
	test.Require().NoError(err)
	ziplineEngine.AssertNotCalled(test.T(), "Sessions", mock.Anything, mock.Anything, mock.Anything)
}

// A provider that sent a bar with no price fails that
// symbol's download with the days. They join the stored-bar check, so the
// create still fails once, naming every symbol: here BBB from the provider
// (no bars stored) and AAA from its stored bars.
func (test *BacktestCommandTest) TestMissingSessionsJoinTheProviderReport() {
	test.createQuarter("AAA", "BBB")
	test.storeBars("AAA", "2024-01-01", "2024-03-31", "2024-02-14")

	_, err := test.download(quarterEnd, &financeServicePB.DownloadHistoricalDataResponse{
		Symbol: "BBB",
		Status: financeServicePB.DownloadHistoricalDataResponse_STATUS_ERROR,
		Error:  "yahoo sent no price for BBB on 2024-02-20, 2024-02-21, 2024-03-05",
	})
	test.Require().EqualError(err, "Couldn't load data for backtest 'oef-2024q1': yahoo is missing prices on trading "+
		"days for 2 symbols. The backtest wasn't saved.\n"+
		"  AAA: 2024-02-14\n"+
		"  BBB: 2024-02-20 to 2024-02-21, 2024-03-05\n"+
		"Try another provider with --provider, or change the backtest dates, then run `fbull backtest create` again.")
	test.requireNothingSaved()
}

// The provider names the stored ticker; the message names the request
// symbol the finance service answered for.
func (test *BacktestCommandTest) TestMissingSessionFromTheProviderAlone() {
	test.createQuarter("XYZ")

	_, err := test.download(quarterEnd, &financeServicePB.DownloadHistoricalDataResponse{
		Symbol: "XYZ",
		Status: financeServicePB.DownloadHistoricalDataResponse_STATUS_ERROR,
		Error:  "yahoo sent no price for XYZ on 2024-02-14",
	})
	test.Require().EqualError(err, "Couldn't load data for backtest 'oef-2024q1': yahoo is missing prices for XYZ on "+
		"2024-02-14, a trading day. The backtest wasn't saved. Try another provider with --provider, or change the "+
		"backtest dates, then run `fbull backtest create` again.")
	test.requireNothingSaved()
}

// A provider that reported no price on a day that isn't a session (Good
// Friday, here) failed that symbol's download, so create fails naming the
// day rather than ingesting without the symbol. It isn't called a trading
// day.
func (test *BacktestCommandTest) TestMissingSessionReportedOnAHolidayFails() {
	test.createQuarter("XYZ")
	test.storeBars("XYZ", "2024-01-01", "2024-03-31")

	_, err := test.download(quarterEnd, &financeServicePB.DownloadHistoricalDataResponse{
		Symbol: "XYZ",
		Status: financeServicePB.DownloadHistoricalDataResponse_STATUS_ERROR,
		Error:  "yahoo sent no price for XYZ on 2024-03-29",
	})
	test.Require().EqualError(err, "Couldn't load data for backtest 'oef-2024q1': yahoo is missing prices for XYZ on "+
		"2024-03-29. The backtest wasn't saved. Try another provider with --provider, or change the "+
		"backtest dates, then run `fbull backtest create` again.")
	test.requireNothingSaved()
}

// A reported holiday next to a reported session reads as one run, and with
// a second symbol the lead doesn't say trading days.
func (test *BacktestCommandTest) TestMissingSessionsReportedAroundAHoliday() {
	test.createQuarter("AAA", "XYZ")
	test.storeBars("AAA", "2024-01-01", "2024-03-31", "2024-02-14")

	_, err := test.download(quarterEnd, &financeServicePB.DownloadHistoricalDataResponse{
		Symbol: "XYZ",
		Status: financeServicePB.DownloadHistoricalDataResponse_STATUS_ERROR,
		Error:  "yahoo sent no price for XYZ on 2024-03-28, 2024-03-29",
	})
	test.Require().EqualError(err, "Couldn't load data for backtest 'oef-2024q1': yahoo is missing prices for "+
		"2 symbols. The backtest wasn't saved.\n"+
		"  AAA: 2024-02-14\n"+
		"  XYZ: 2024-03-28 to 2024-03-29\n"+
		"Try another provider with --provider, or change the backtest dates, then run `fbull backtest create` again.")
	test.requireNothingSaved()
}

// storeBarsOn stores a yahoo bar for symbol on each day, at 14:30 UTC.
func (test *BacktestCommandTest) storeBarsOn(symbol string, days ...string) {
	ctx := context.Background()
	_, err := test.db.Exec(ctx, `INSERT INTO asset (symbol, name) VALUES ($1, $1) ON CONFLICT DO NOTHING`, symbol)
	test.Require().NoError(err)
	for _, day := range days {
		at, err := time.Parse(time.DateOnly, day)
		test.Require().NoError(err)
		_, err = test.db.Exec(ctx, `
			INSERT INTO ohlc (symbol, open, high, low, close, volume, time, provider)
			VALUES ($1, 10, 11, 9, 10, 100, $2, 'yahoo')`, symbol, at.Add(14*time.Hour+30*time.Minute))
		test.Require().NoError(err)
	}
}

// The span is the symbol's first and last stored bar over all time, clipped
// to the period. Bars before the start mean the first two period sessions
// are missing, not a listing.
func (test *BacktestCommandTest) TestMissingSessionsAtThePeriodStartWithBarsBefore() {
	test.createQuarter("XYZ")
	test.storeBarsOn("XYZ", "2023-12-28", "2023-12-29")
	test.storeBars("XYZ", "2024-01-04", "2024-03-31")

	_, err := test.downloadQuarter()
	test.Require().ErrorContains(err, "yahoo is missing prices for XYZ on 2 trading days: 2024-01-02 to 2024-01-03. ")
	test.requireNothingSaved()
}

// Bars after the end mean the last period session is missing, not a
// delisting.
func (test *BacktestCommandTest) TestMissingSessionAtThePeriodEndWithBarsAfter() {
	test.createQuarter("XYZ")
	test.storeBars("XYZ", "2024-01-01", "2024-03-27")
	test.storeBarsOn("XYZ", "2024-04-01")

	_, err := test.downloadQuarter()
	test.Require().ErrorContains(err, "yahoo is missing prices for XYZ on 2024-03-28, a trading day. ")
	test.requireNothingSaved()
}

// A create canceled during the check returns context.Canceled itself,
// unwrapped, like every other canceled create, and never the data service
// copy. The backtest is still removed.
func (test *BacktestCommandTest) TestCanceledMissingSessionCheckReturnsTheContextError() {
	test.createQuarter("XYZ")
	test.storeBars("XYZ", "2024-01-01", "2024-03-31")
	clientStream := new(financeServicePB.MockMarketdata_DownloadHistoricalDataClient[interface{}])
	clientStream.On("Recv", mock.Anything).Return(nil, io.EOF)
	test.mdclient.On("DownloadHistoricalData", mock.Anything, mock.Anything).Return(clientStream, nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ziplineEngine := new(engine.MockEngine)
	ziplineEngine.On("Sessions", mock.Anything, quarterStart, quarterEnd).
		Run(func(mock.Arguments) { cancel() }).
		Return(nil, status.Error(codes.Canceled, "context canceled"))
	msg := test.downloadMessage(quarterBacktest)
	msg.On("Call", mock.Anything, dependency.GetEngineKey).Return(ziplineEngine, nil)

	err := command.DownloadFinancialData(ctx, msg)
	test.Require().ErrorIs(err, context.Canceled)
	test.Require().Equal(context.Canceled, err)
	test.NotContains(err.Error(), "data service")
	test.requireNothingSaved()
}

// A create canceled during the ingest does the same.
func (test *BacktestCommandTest) TestCanceledIngestReturnsTheContextError() {
	test.createQuarter("AAA")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ziplineEngine := new(engine.MockEngine)
	ziplineEngine.On("Ingest", mock.Anything, mock.Anything, mock.Anything).
		Run(func(mock.Arguments) { cancel() }).
		Return(nil, fmt.Errorf("error ingesting: %w", status.Error(codes.Canceled, "context canceled")))

	err := command.Ingest(ctx, test.ingestMessage(quarterBacktest, ziplineEngine))
	test.Require().ErrorIs(err, context.Canceled)
	test.Require().Equal(context.Canceled, err)
	test.NotContains(err.Error(), "data service")
	test.requireNothingSaved()
}

// Zipline's MissingSessionsError reaches the broker as DATA_LOSS. It reads
// the same as the download check's sentence, and the backtest is removed.
func (test *BacktestCommandTest) TestIngestMissingSessionsUseTheCreateSentence() {
	test.createQuarter("AAA", "BBB")
	ziplineEngine := new(engine.MockEngine)
	ziplineEngine.On("Ingest", mock.Anything, mock.Anything, mock.Anything).Return(nil, fmt.Errorf("error ingesting: %w",
		status.Error(codes.DataLoss, "missing prices on trading days: AAA 2024-02-14; BBB 2024-02-20,2024-02-21,2024-03-05"+
			"\nDetails: MissingSessionsError: missing prices on trading days: …")))
	ziplineEngine.On("Sessions", mock.Anything, quarterStart, quarterEnd).Return(xnysSessions(quarterEnd), nil)

	err := command.Ingest(context.TODO(), test.ingestMessage(quarterBacktest, ziplineEngine))
	test.Require().EqualError(err, "Couldn't load data for backtest 'oef-2024q1': yahoo is missing prices on trading "+
		"days for 2 symbols. The backtest wasn't saved.\n"+
		"  AAA: 2024-02-14\n"+
		"  BBB: 2024-02-20 to 2024-02-21, 2024-03-05\n"+
		"Try another provider with --provider, or change the backtest dates, then run `fbull backtest create` again.")
	test.requireNothingSaved()
}

// Another DATA_LOSS text keeps the ingest failure copy.
func (test *BacktestCommandTest) TestIngestOtherDataLossKeepsTheIngestCopy() {
	test.createQuarter("AAA")
	ziplineEngine := new(engine.MockEngine)
	ziplineEngine.On("Ingest", mock.Anything, mock.Anything, mock.Anything).
		Return(nil, status.Error(codes.DataLoss, "something else"))

	err := command.Ingest(context.TODO(), test.ingestMessage(quarterBacktest, ziplineEngine))
	test.Require().ErrorContains(err, "the data service failed while ingesting prices.")
	ziplineEngine.AssertNotCalled(test.T(), "Sessions", mock.Anything, mock.Anything, mock.Anything)
}

// A DATA_LOSS list the sentence can't name in full falls back to the ingest
// copy, so the list and its symbol count are never short.
func (test *BacktestCommandTest) requireIngestCopyFor(reason string) {
	test.createQuarter("AAA")
	ziplineEngine := new(engine.MockEngine)
	ziplineEngine.On("Ingest", mock.Anything, mock.Anything, mock.Anything).
		Return(nil, status.Error(codes.DataLoss, reason))
	ziplineEngine.On("Sessions", mock.Anything, quarterStart, quarterEnd).Return(xnysSessions(quarterEnd), nil)

	err := command.Ingest(context.TODO(), test.ingestMessage(quarterBacktest, ziplineEngine))
	test.Require().ErrorContains(err, "the data service failed while ingesting prices.")
	test.NotContains(err.Error(), "is missing prices")
	test.requireNothingSaved()
}

// Zipline named a ticker that isn't in the backtest.
func (test *BacktestCommandTest) TestIngestMissingSessionsOfAnotherTickerKeepTheIngestCopy() {
	test.requireIngestCopyFor("missing prices on trading days: AAA 2024-02-14; ZZZ 2024-02-01")
}

// Zipline named a day that isn't a session (Good Friday).
func (test *BacktestCommandTest) TestIngestMissingSessionsOnAHolidayKeepTheIngestCopy() {
	test.requireIngestCopyFor("missing prices on trading days: AAA 2024-02-14,2024-03-29")
}
