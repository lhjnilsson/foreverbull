package command_test

import (
	"context"
	"io"
	"time"

	"github.com/lhjnilsson/foreverbull/pkg/backtest/engine"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/repository"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/stream/command"
	"github.com/lhjnilsson/foreverbull/pkg/pb"
	"github.com/lhjnilsson/foreverbull/pkg/pb/backtest"
	financeServicePB "github.com/lhjnilsson/foreverbull/pkg/pb/finance/service"
	"github.com/stretchr/testify/mock"
)

// The broker reads the clock once, before 16:30 New York on 2024-03-28.
// The notice, the download's cut-off, the stored bars, and the bundle's
// end all stop at 2024-03-27. The requested end stays 2024-03-28, and the
// download is not asked for that open session.
func (test *BacktestCommandTest) TestOneCutoffAgreesAcrossNoticeBarsAndBundle() {
	test.fixClock(beforeTheClose)
	requested := time.Date(2024, 3, 28, 0, 0, 0, 0, time.UTC)
	capped := time.Date(2024, 3, 27, 0, 0, 0, 0, time.UTC)
	test.createClosedThrough("yahoo", requested, "2024-03-27")

	var sent *financeServicePB.DownloadHistoricalDataRequest
	clientStream := new(financeServicePB.MockMarketdata_DownloadHistoricalDataClient[interface{}])
	clientStream.On("Recv", mock.Anything).Return(nil, io.EOF)
	test.mdclient.On("DownloadHistoricalData", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		sent = args.Get(1).(*financeServicePB.DownloadHistoricalDataRequest)
	}).Return(clientStream, nil)

	ziplineEngine := new(engine.MockEngine)
	ziplineEngine.On("Sessions", mock.Anything, mock.Anything, mock.Anything).Return(
		func(_ context.Context, start, end time.Time) ([]time.Time, error) {
			return sessionsBetween(start, end), nil
		})
	msg := test.downloadMessage(quarterBacktest)
	useEngine(msg, ziplineEngine)
	err := command.DownloadFinancialData(context.Background(), msg)
	test.requireNotice(err, "2024-03-28 isn't included because its trading session hasn't closed yet "+
		"(sessions count as closed at 16:30 New York time).")
	test.Require().NotNil(sent)
	test.Equal("2024-03-27", pb.DateToDateString(sent.GetEndDate()))
	test.Equal("2024-03-27", pb.DateToDateString(sent.GetSessionCutoff()))

	var onExcluded int
	test.Require().NoError(test.db.QueryRow(context.Background(), `
		SELECT count(*) FROM ohlc
		 WHERE provider = 'yahoo' AND time::date = '2024-03-28'`).Scan(&onExcluded))
	test.Zero(onExcluded)
	var last string
	test.Require().NoError(test.db.QueryRow(context.Background(), `
		SELECT max(time)::date::text FROM ohlc WHERE provider = 'yahoo'`).Scan(&last))
	test.Equal("2024-03-27", last)

	backtests := repository.Backtest{Conn: test.db}
	stored, err := backtests.Get(context.Background(), quarterBacktest)
	test.Require().NoError(err)
	test.Equal("2024-03-28", pb.DateToDateString(stored.GetPeriods()[len(stored.GetPeriods())-1].GetEndDate()))
	closed, err := backtests.ClosedEnd(context.Background(), quarterBacktest)
	test.Require().NoError(err)
	test.True(closed.Equal(capped))

	var bundled *backtest.Backtest
	ingestEngine := new(engine.MockEngine)
	ingestEngine.On("Ingest", mock.Anything, mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		bundled = args.Get(1).(*backtest.Backtest)
	}).Return([]string(nil), nil)
	err = command.Ingest(context.Background(), test.ingestMessage(quarterBacktest, ingestEngine))
	test.Require().NoError(err)
	test.Require().NotNil(bundled)
	periods := bundled.GetPeriods()
	test.Equal("2024-03-27", pb.DateToDateString(periods[len(periods)-1].GetEndDate()))
	again, err := backtests.Get(context.Background(), quarterBacktest)
	test.Require().NoError(err)
	test.Equal("2024-03-28", pb.DateToDateString(again.GetPeriods()[len(again.GetPeriods())-1].GetEndDate()))
}

// A bundle is not built when the download never recorded a cut-off. Ingest
// does not read the clock to invent one.
func (test *BacktestCommandTest) TestIngestFailsWhenTheCutoffWasNotRecorded() {
	test.createQuarter("AAA")
	ziplineEngine := new(engine.MockEngine)
	msg := test.ingestMessageRaw(quarterBacktest, ziplineEngine)

	err := command.Ingest(context.Background(), msg)
	test.Require().EqualError(err, "Couldn't load data for backtest 'oef-2024q1': foreverbull didn't record which "+
		"trading session it stops at. The backtest wasn't saved. Run `fbull backtest create` again.")
	ziplineEngine.AssertNotCalled(test.T(), "Ingest", mock.Anything, mock.Anything, mock.Anything)
	test.requireNothingSaved()
}
