package command_test

import (
	"context"

	"github.com/jackc/pgx/v5"
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

// A provider timeout fails create with its own sentence, whether it comes
// per symbol or as the download's time limit, and the backtest is removed.
func (test *BacktestCommandTest) TestDownloadFinancialDataNamesProviderTimeout() {
	for name, timedOut := range map[string]func(*financeServicePB.MockMarketdata_DownloadHistoricalDataClient[interface{}]){
		"per symbol": func(clientStream *financeServicePB.MockMarketdata_DownloadHistoricalDataClient[interface{}]) {
			clientStream.On("Recv", mock.Anything).Return(&financeServicePB.DownloadHistoricalDataResponse{
				Symbol: "AAPL",
				Status: financeServicePB.DownloadHistoricalDataResponse_STATUS_ERROR,
				Error:  "alpaca took too long to respond for AAPL",
			}, nil).Once()
		},
		"time limit": func(clientStream *financeServicePB.MockMarketdata_DownloadHistoricalDataClient[interface{}]) {
			clientStream.On("Recv", mock.Anything).Return(&financeServicePB.DownloadHistoricalDataResponse{
				Symbol: "S01",
				Status: financeServicePB.DownloadHistoricalDataResponse_STATUS_OK,
			}, nil).Once()
			clientStream.On("Recv", mock.Anything).
				Return(nil, status.Error(codes.DeadlineExceeded, "alpaca took too long to respond for AAPL")).Once()
		},
	} {
		test.Run(name, func() {
			test.createOEF(98)
			clientStream := new(financeServicePB.MockMarketdata_DownloadHistoricalDataClient[interface{}])
			timedOut(clientStream)
			test.mdclient.On("DownloadHistoricalData", mock.Anything, mock.Anything).Return(clientStream, nil).Once()

			err := command.DownloadFinancialData(context.TODO(), test.downloadMessage("oef-2024q1"))

			test.Require().EqualError(err, providerTimeoutCopy)
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
