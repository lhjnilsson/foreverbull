package command_test

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/engine"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/repository"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/stream/command"
	"github.com/lhjnilsson/foreverbull/pkg/pb"
	"github.com/lhjnilsson/foreverbull/pkg/pb/backtest"
	financeServicePB "github.com/lhjnilsson/foreverbull/pkg/pb/finance/service"
	"github.com/stretchr/testify/mock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const blockedRetrySentence = "Stooq blocked the download for AAPL (it returned a bot-check page instead of data). Retry with --provider yahoo."

// refuseBacktestDelete makes the next delete of a backtest fail. The row
// stays, which is the case these tests show to the user.
func (test *BacktestCommandTest) refuseBacktestDelete() {
	_, err := test.db.Exec(context.Background(), `
CREATE OR REPLACE FUNCTION refuse_backtest_delete() RETURNS trigger AS $$
BEGIN
	RAISE EXCEPTION 'backtest delete refused';
END $$ LANGUAGE plpgsql;
DROP TRIGGER IF EXISTS refuse_backtest_delete ON backtest;
CREATE TRIGGER refuse_backtest_delete BEFORE DELETE ON backtest
	FOR EACH ROW EXECUTE FUNCTION refuse_backtest_delete();`)
	test.Require().NoError(err)
	test.T().Cleanup(func() {
		_, err := test.db.Exec(context.Background(), `DROP TRIGGER IF EXISTS refuse_backtest_delete ON backtest`)
		test.Require().NoError(err)
	})
}

func (test *BacktestCommandTest) downloadUntilBlocked(name string) error {
	backtests := repository.Backtest{Conn: test.db}
	err := backtests.UpdatePeriod(context.TODO(), name, &backtest.Backtest_Period{
		StartDate: &pb.Date{Year: 2022, Month: 1, Day: 1},
		EndDate:   &pb.Date{Year: 2022, Month: 12, Day: 31},
		Assets: []*backtest.Backtest_Period_Asset{
			{Symbol: "AAPL", Enabled: true},
		},
	})
	test.Require().NoError(err)
	clientStream := new(financeServicePB.MockMarketdata_DownloadHistoricalDataClient[interface{}])
	clientStream.On("Recv", mock.Anything).Return(nil, status.Error(codes.FailedPrecondition, blockedRetrySentence))
	test.mdclient.On("DownloadHistoricalData", mock.Anything, mock.Anything).Return(clientStream, nil)
	msg := test.downloadMessage(name)
	useEngine(msg, weekdayEngine())
	return command.DownloadFinancialData(context.TODO(), msg)
}

// A blocked download whose backtest was removed keeps the provider's retry
// sentence, and the name can be used again.
func (test *BacktestCommandTest) TestBlockedDownloadKeepsTheRetrySentenceWhenRemoved() {
	err := test.downloadUntilBlocked("test-backtest")
	test.Require().EqualError(err, blockedRetrySentence)

	_, err = (&repository.Backtest{Conn: test.db}).Get(context.TODO(), "test-backtest")
	test.Require().ErrorIs(err, pgx.ErrNoRows)
}

// A blocked download whose backtest could not be removed says to delete it
// first. The retry sentence stays, after that step, and the name is still taken.
func (test *BacktestCommandTest) TestBlockedDownloadSaysToDeleteWhenRemovalFails() {
	test.refuseBacktestDelete()

	err := test.downloadUntilBlocked("test-backtest")
	test.Require().EqualError(err,
		"The backtest was partly saved and couldn't be removed. Run `fbull backtest delete test-backtest` first. "+
			blockedRetrySentence)
	test.NotContains(err.Error(), "refused")

	stored, err := (&repository.Backtest{Conn: test.db}).Get(context.TODO(), "test-backtest")
	test.Require().NoError(err)
	test.Equal("test-backtest", stored.Name)
}

// A canceled ingest whose backtest was removed reports the cancel itself.
func (test *BacktestCommandTest) TestCanceledIngestReportsCancellationWhenRemoved() {
	test.createQuarter("AAA")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ziplineEngine := new(engine.MockEngine)
	ziplineEngine.On("Ingest", mock.Anything, mock.Anything, mock.Anything).
		Run(func(mock.Arguments) { cancel() }).
		Return(nil, fmt.Errorf("error ingesting: %w", status.Error(codes.Canceled, "context canceled")))

	err := command.Ingest(ctx, test.ingestMessage(quarterBacktest, ziplineEngine))
	test.Require().EqualError(err, "context canceled")
	test.Require().Equal(context.Canceled, err)
	test.NotContains(err.Error(), "data service")
	test.requireNothingSaved()
}

// A canceled ingest whose backtest could not be removed says to delete it
// first. The cancel alone would hide that the name is still taken.
func (test *BacktestCommandTest) TestCanceledIngestSaysToDeleteWhenRemovalFails() {
	test.createQuarter("AAA")
	test.refuseBacktestDelete()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ziplineEngine := new(engine.MockEngine)
	ziplineEngine.On("Ingest", mock.Anything, mock.Anything, mock.Anything).
		Run(func(mock.Arguments) { cancel() }).
		Return(nil, fmt.Errorf("error ingesting: %w", status.Error(codes.Canceled, "context canceled")))

	err := command.Ingest(ctx, test.ingestMessage(quarterBacktest, ziplineEngine))
	test.Require().EqualError(err,
		"Couldn't load data for backtest 'oef-2024q1': the create was canceled. "+
			"The backtest was partly saved and couldn't be removed. Run `fbull backtest delete oef-2024q1` first. "+
			"Run `fbull backtest create` again.")
	test.NotContains(err.Error(), "refused")
	test.NotContains(err.Error(), "data service")
	test.NotErrorIs(err, context.Canceled)
	test.requireKept()
}
