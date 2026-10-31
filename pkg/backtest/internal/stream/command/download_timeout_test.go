package command_test

import (
	"context"
	"io"

	"github.com/jackc/pgx/v5"
	"github.com/lhjnilsson/foreverbull/internal/stream"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/repository"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/stream/command"
	financeServicePB "github.com/lhjnilsson/foreverbull/pkg/pb/finance/service"
	"github.com/stretchr/testify/mock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// providerTimeoutCopy is the agreed create sentence for a provider that took
// too long, word for word.
const providerTimeoutCopy = "Couldn't load data for backtest 'oef-2024q1': alpaca took too long to respond for AAPL. " +
	"The backtest wasn't saved. Try again later, or use another provider with --provider."

// A per-symbol provider timeout fails create with its own sentence, and the
// backtest is removed.
func (test *BacktestCommandTest) TestDownloadFinancialDataNamesProviderTimeout() {
	test.createOEF(98)
	clientStream := new(financeServicePB.MockMarketdata_DownloadHistoricalDataClient[interface{}])
	clientStream.On("Recv", mock.Anything).Return(&financeServicePB.DownloadHistoricalDataResponse{
		Symbol: "AAPL",
		Status: financeServicePB.DownloadHistoricalDataResponse_STATUS_ERROR,
		Error:  "alpaca took too long to respond for AAPL",
	}, nil).Once()
	test.mdclient.On("DownloadHistoricalData", mock.Anything, mock.Anything).Return(clientStream, nil).Once()

	err := command.DownloadFinancialData(context.TODO(), test.downloadMessage("oef-2024q1"))

	test.Require().EqualError(err, providerTimeoutCopy)
	backtests := repository.Backtest{Conn: test.db}
	_, err = backtests.Get(context.TODO(), "oef-2024q1")
	test.Require().ErrorIs(err, pgx.ErrNoRows)
}

// The finance service's time limit gets its own sentence, not the provider
// timeout one. It counts the symbols answered so far out of the distinct
// total, and the backtest is removed.
func (test *BacktestCommandTest) TestDownloadFinancialDataNamesTheTimeLimit() {
	for limit, span := range map[string]string{"2h0m0s": "2-hour", "1h0m0s": "1-hour", "1h30m0s": "90-minute"} {
		test.Run(limit, func() {
			test.createOEF(98)
			clientStream := new(financeServicePB.MockMarketdata_DownloadHistoricalDataClient[interface{}])
			for _, symbol := range []string{"S01", "S02"} {
				clientStream.On("Recv", mock.Anything).Return(&financeServicePB.DownloadHistoricalDataResponse{
					Symbol: symbol,
					Status: financeServicePB.DownloadHistoricalDataResponse_STATUS_OK,
				}, nil).Once()
			}
			clientStream.On("Recv", mock.Anything).
				Return(nil, status.Error(codes.DeadlineExceeded, "the download hit its time limit of "+limit+" at S03")).Once()
			test.mdclient.On("DownloadHistoricalData", mock.Anything, mock.Anything).Return(clientStream, nil).Once()

			err := command.DownloadFinancialData(context.TODO(), test.downloadMessage("oef-2024q1"))

			// 98 members and the benchmark.
			test.Require().EqualError(err, "Couldn't load data for backtest 'oef-2024q1': the download hit the "+span+" limit "+
				"after 2 of 99 symbols. The backtest wasn't saved. Split the symbols across several backtests, "+
				"then run `fbull backtest create` again.")
			backtests := repository.Backtest{Conn: test.db}
			_, err = backtests.Get(context.TODO(), "oef-2024q1")
			test.Require().ErrorIs(err, pgx.ErrNoRows)
		})
	}
}

// The client's own deadline is not the provider's fault: it carries other
// text, so create keeps the generic failed-download sentence.
func (test *BacktestCommandTest) TestDownloadFinancialDataKeepsOtherDeadlinesGeneric() {
	test.createOEF(98)
	clientStream := new(financeServicePB.MockMarketdata_DownloadHistoricalDataClient[interface{}])
	clientStream.On("Recv", mock.Anything).Return(nil, status.Error(codes.DeadlineExceeded, "context deadline exceeded")).Once()
	test.mdclient.On("DownloadHistoricalData", mock.Anything, mock.Anything).Return(clientStream, nil)

	err := command.DownloadFinancialData(context.TODO(), test.downloadMessage("oef-2024q1"))

	test.Require().EqualError(err, "Downloading prices failed: context deadline exceeded. Nothing was changed. Run the same create again.")
}

// Create reports download progress on the command's ctx before the first
// symbol is answered, counting the distinct symbols it asks for.
func (test *BacktestCommandTest) TestDownloadFinancialDataReportsProgress() {
	test.createOEF(98)
	clientStream := new(financeServicePB.MockMarketdata_DownloadHistoricalDataClient[interface{}])
	clientStream.On("Recv", mock.Anything).Return(nil, io.EOF).Once()
	test.mdclient.On("DownloadHistoricalData", mock.Anything, mock.Anything).Return(clientStream, nil)

	reports := make([]string, 0)
	ctx := stream.WithProgress(context.TODO(), func(text string) { reports = append(reports, text) })
	err := command.DownloadFinancialData(ctx, test.downloadMessage("oef-2024q1"))
	// No bars are stored, so create then fails the missing-session check;
	// the report sent before it is what this test checks.
	test.Require().ErrorContains(err, "missing prices on trading days")
	// 98 members and the benchmark.
	test.Equal([]string{"Downloading prices: 0 of 99 symbols"}, reports)
}
