package command_test

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/lhjnilsson/foreverbull/internal/stream"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/repository"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/stream/command"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/stream/dependency"
	bs "github.com/lhjnilsson/foreverbull/pkg/backtest/stream"
	financepkg "github.com/lhjnilsson/foreverbull/pkg/finance"
	"github.com/lhjnilsson/foreverbull/pkg/pb"
	"github.com/lhjnilsson/foreverbull/pkg/pb/backtest"
	"github.com/stretchr/testify/mock"
	"google.golang.org/protobuf/proto"
)

func (test *BacktestCommandTest) quoteLondon() func() {
	return financepkg.SetQuoteReader(func(_ context.Context, _, _, symbol string) (financepkg.Quote, error) {
		switch strings.ToUpper(symbol) {
		case "VOD.L", "^FTSE":
			return financepkg.Quote{Exchange: "LSE", Currency: "GBP", Reported: true}, nil
		case "BMW.DE":
			return financepkg.Quote{Exchange: "GER", Currency: "EUR", Reported: true}, nil
		case "XYZ":
			return financepkg.Quote{Exchange: "NMS", Currency: "CAD", Reported: true}, nil
		default:
			return financepkg.Quote{Exchange: "NMS", Currency: "USD", Reported: true}, nil
		}
	})
}

func (test *BacktestCommandTest) createFile(b *backtest.Backtest) error {
	message := new(stream.MockMessage)
	message.On("ParsePayload", &bs.CreateBacktestFromBacktestCommand{}).Return(nil).Run(func(args mock.Arguments) {
		payload := args.Get(0).(*bs.CreateBacktestFromBacktestCommand)
		bytes, err := proto.Marshal(b)
		test.Require().NoError(err)
		payload.Backtest = bytes
	})
	message.On("MustGet", stream.DBDep).Return(test.db)
	allowCalendar(message)
	return command.CreateBacktestFromBacktest(context.Background(), message)
}

func (test *BacktestCommandTest) periodAssets(symbols ...string) []*backtest.Backtest_Period {
	assets := make([]*backtest.Backtest_Period_Asset, len(symbols))
	for i, symbol := range symbols {
		assets[i] = &backtest.Backtest_Period_Asset{Symbol: symbol, Enabled: true}
	}
	return []*backtest.Backtest_Period{{
		StartDate: &pb.Date{Year: 2024, Month: 1, Day: 1},
		EndDate:   &pb.Date{Year: 2024, Month: 3, Day: 31},
		Assets:    assets,
	}}
}

func (test *BacktestCommandTest) assertNothingStored(name string) {
	test.T().Helper()
	_, err := (&repository.Backtest{Conn: test.db}).Get(context.Background(), name)
	test.Require().ErrorIs(err, pgx.ErrNoRows)
	var assets, bars int
	test.Require().NoError(test.db.QueryRow(context.Background(), `SELECT count(*) FROM asset`).Scan(&assets))
	test.Require().NoError(test.db.QueryRow(context.Background(), `SELECT count(*) FROM ohlc`).Scan(&bars))
	test.Zero(assets)
	test.Zero(bars)
}

func (test *BacktestCommandTest) TestCreateRefusesALondonListing() {
	restore := test.quoteLondon()
	defer restore()
	err := test.createFile(&backtest.Backtest{
		Name: "uk-2024", Benchmark: "SPY", Provider: "yahoo",
		Periods: test.periodAssets("VOD.L"),
	})
	test.Require().EqualError(err, "Couldn't create backtest 'uk-2024': VOD.L is listed outside the US (London, GBP). Global markets aren't supported yet. The backtest wasn't saved. Remove VOD.L, then run `fbull backtest create` again.")
	test.assertNothingStored("uk-2024")
}

func (test *BacktestCommandTest) TestCreateRefusesEverySymbol() {
	restore := test.quoteLondon()
	defer restore()
	err := test.createFile(&backtest.Backtest{
		Name: "uk-2024", Benchmark: "SPY", Provider: "yahoo",
		Periods: test.periodAssets("XYZ", "VOD.L"),
	})
	test.Require().EqualError(err, "Couldn't create backtest 'uk-2024': 2 symbols aren't US-listed in USD: VOD.L (London, GBP), XYZ (yahoo prices it in CAD). Global markets aren't supported yet. The backtest wasn't saved. Remove them, then run `fbull backtest create` again.")
	test.assertNothingStored("uk-2024")
}

func (test *BacktestCommandTest) TestCreateRefusesTheBenchmark() {
	restore := test.quoteLondon()
	defer restore()
	err := test.createFile(&backtest.Backtest{
		Name: "uk-2024", Benchmark: "^FTSE", Provider: "yahoo",
		Periods: test.periodAssets("AAPL"),
	})
	test.Require().EqualError(err, "Couldn't create backtest 'uk-2024': the benchmark ^FTSE is listed outside the US (London, GBP). Global markets aren't supported yet. The backtest wasn't saved. Set `benchmark` in the backtest file to a US-listed symbol such as SPY, then run `fbull backtest create` again.")
	test.assertNothingStored("uk-2024")
}

func (test *BacktestCommandTest) TestCreateRefusesACurrencyWithoutASuffix() {
	restore := test.quoteLondon()
	defer restore()
	err := test.createFile(&backtest.Backtest{
		Name: "uk-2024", Benchmark: "SPY", Provider: "yahoo",
		Periods: test.periodAssets("XYZ"),
	})
	test.Require().EqualError(err, "Couldn't create backtest 'uk-2024': yahoo prices XYZ in CAD, not USD. Global markets aren't supported yet. The backtest wasn't saved. Remove XYZ, then run `fbull backtest create` again.")
	test.assertNothingStored("uk-2024")
}

func (test *BacktestCommandTest) TestCreateBacktestFromETFRefusesALondonListing() {
	restore := test.quoteLondon()
	defer restore()
	message := new(stream.MockMessage)
	message.On("ParsePayload", &bs.CreateBacktestFromETFCommand{}).Return(nil).Run(func(args mock.Arguments) {
		payload := args.Get(0).(*bs.CreateBacktestFromETFCommand)
		payload.BacktestName = "uk-2024"
		payload.ETFName = "iShares S&P 100"
		payload.StartDate = "2024-01-01"
		payload.EndDate = "2024-03-31"
		payload.Provider = "yahoo"
	})
	message.On("MustGet", stream.DBDep).Return(test.db)
	message.On("MustGet", dependency.GetMarketdataClient).Return(test.mdclient)
	message.On("MustGet", dependency.GetETFScraper).Return(test.etfscraper)
	allowCalendar(message)

	start := test.date("2024-01-01")
	end := test.date("2024-03-31")
	seed := test.date("2023-12-31")
	vod := dependency.Asset{Symbol: "VOD.L", Name: "Vodafone", Sector: "Telecom", AssetType: "Equity", ISIN: "GB00BH4HKS39"}
	test.etfscraper.On("ListSnapshotDates", "iShares S&P 100", start, end).Return([]time.Time{start}, nil)
	test.etfscraper.On("ListSnapshotDates", "iShares S&P 100", test.date("2023-10-01"), start).Return([]time.Time{seed}, nil)
	test.etfscraper.On("GetSnapshot", "iShares S&P 100", seed).Return([]dependency.Asset{vod}, nil)
	test.etfscraper.On("GetSnapshot", "iShares S&P 100", start).Return([]dependency.Asset{vod}, nil)
	test.etfscraper.On("GetBenchmark", "iShares S&P 100").Return("SPY", nil)

	err := command.CreateBacktestFromETF(context.Background(), message)
	test.Require().EqualError(err, "Couldn't create backtest 'uk-2024': VOD.L is listed outside the US (London, GBP). Global markets aren't supported yet. The backtest wasn't saved. Remove VOD.L, then run `fbull backtest create` again.")
	test.assertNothingStored("uk-2024")
	test.mdclient.AssertNotCalled(test.T(), "GetAsset", mock.Anything, mock.Anything)
}

func (test *BacktestCommandTest) TestDownloadAndIngestRefuseALondonListing() {
	restore := test.quoteLondon()
	defer restore()
	backtests := repository.Backtest{Conn: test.db}
	_, _, err := backtests.Create(context.Background(), &backtest.Backtest{
		Name: "uk-2024", Benchmark: "SPY", Provider: "yahoo",
		Periods: test.periodAssets("VOD.L"),
	})
	test.Require().NoError(err)

	download := new(stream.MockMessage)
	download.On("MustGet", stream.DBDep).Return(test.db)
	download.On("MustGet", dependency.GetMarketdataClient).Return(test.mdclient)
	download.On("ParsePayload", &bs.DownloadFinancialDataCommand{}).Return(nil).Run(func(args mock.Arguments) {
		payload := args.Get(0).(*bs.DownloadFinancialDataCommand)
		payload.BacktestName = "uk-2024"
	})
	err = command.DownloadFinancialData(context.Background(), download)
	test.Require().EqualError(err, "Couldn't create backtest 'uk-2024': VOD.L is listed outside the US (London, GBP). Global markets aren't supported yet. The backtest wasn't saved. Remove VOD.L, then run `fbull backtest create` again.")
	test.assertNothingStored("uk-2024")
	test.mdclient.AssertNotCalled(test.T(), "DownloadHistoricalData", mock.Anything, mock.Anything)

	_, _, err = backtests.Create(context.Background(), &backtest.Backtest{
		Name: "uk-2024", Benchmark: "SPY", Provider: "yahoo",
		Periods: test.periodAssets("VOD.L"),
	})
	test.Require().NoError(err)
	ingest := new(stream.MockMessage)
	ingest.On("MustGet", stream.DBDep).Return(test.db)
	ingest.On("ParsePayload", &bs.IngestCommand{}).Return(nil).Run(func(args mock.Arguments) {
		payload := args.Get(0).(*bs.IngestCommand)
		payload.BacktestName = "uk-2024"
	})
	err = command.Ingest(context.Background(), ingest)
	test.Require().EqualError(err, "Couldn't create backtest 'uk-2024': VOD.L is listed outside the US (London, GBP). Global markets aren't supported yet. The backtest wasn't saved. Remove VOD.L, then run `fbull backtest create` again.")
	test.assertNothingStored("uk-2024")
}

func (test *BacktestCommandTest) TestCreateLookupFailureDeletesAStoredBacktest() {
	restore := financepkg.SetQuoteReader(func(_ context.Context, _, _, symbol string) (financepkg.Quote, error) {
		if symbol == "XYZ" {
			return financepkg.Quote{}, context.DeadlineExceeded
		}
		return financepkg.Quote{Exchange: "NMS", Currency: "USD", Reported: true}, nil
	})
	defer restore()
	backtests := repository.Backtest{Conn: test.db}
	_, _, err := backtests.Create(context.Background(), &backtest.Backtest{
		Name: "uk-2024", Benchmark: "SPY", Provider: "yahoo",
		Periods: test.periodAssets("XYZ"),
	})
	test.Require().NoError(err)
	test.refuseBacktestDelete()
	download := new(stream.MockMessage)
	download.On("MustGet", stream.DBDep).Return(test.db)
	download.On("MustGet", dependency.GetMarketdataClient).Return(test.mdclient)
	download.On("ParsePayload", &bs.DownloadFinancialDataCommand{}).Return(nil).Run(func(args mock.Arguments) {
		payload := args.Get(0).(*bs.DownloadFinancialDataCommand)
		payload.BacktestName = "uk-2024"
	})
	err = command.DownloadFinancialData(context.Background(), download)
	test.Require().EqualError(err, "Couldn't create backtest 'uk-2024': yahoo didn't respond when checking where XYZ is listed. The backtest was partly saved and couldn't be removed. Run `fbull backtest delete uk-2024` first. Wait a few minutes, then run `fbull backtest create` again.")
	_, err = backtests.Get(context.Background(), "uk-2024")
	test.Require().NoError(err)
}

func (test *BacktestCommandTest) TestCreateAcceptsAnUnknownSymbol() {
	restore := financepkg.SetQuoteReader(func(_ context.Context, _, _, symbol string) (financepkg.Quote, error) {
		if symbol == "NOPE" {
			return financepkg.Quote{}, errUnknownListing()
		}
		return financepkg.Quote{Exchange: "NMS", Currency: "USD", Reported: true}, nil
	})
	defer restore()
	err := test.createFile(&backtest.Backtest{
		Name: "us-2024", Benchmark: "SPY", Provider: "yahoo",
		Periods: test.periodAssets("NOPE"),
	})
	test.Require().NoError(err)
	stored, err := (&repository.Backtest{Conn: test.db}).Get(context.Background(), "us-2024")
	test.Require().NoError(err)
	test.Equal("NOPE", stored.GetPeriods()[0].GetAssets()[0].GetSymbol())
	var checked bool
	test.Require().NoError(test.db.QueryRow(context.Background(), `SELECT markets_checked FROM backtest WHERE name = 'us-2024'`).Scan(&checked))
	test.False(checked)
}

func errUnknownListing() error {
	return fmt.Errorf("yahoo has no quote for NOPE: %w", financepkg.ErrUnknownSymbol)
}

func (test *BacktestCommandTest) TestCreateStillStoresAUSBacktest() {
	err := test.createFile(&backtest.Backtest{
		Name: "us-2024", Benchmark: "SPY", Provider: "yahoo",
		Periods: test.periodAssets("AAPL"),
	})
	test.Require().NoError(err)
	stored, err := (&repository.Backtest{Conn: test.db}).Get(context.Background(), "us-2024")
	test.Require().NoError(err)
	test.Equal("SPY", stored.GetBenchmark())
	test.Equal("AAPL", stored.GetPeriods()[0].GetAssets()[0].GetSymbol())
}
