package command_test

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/lhjnilsson/foreverbull/internal/stream"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/repository"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/stream/command"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/stream/dependency"
	bs "github.com/lhjnilsson/foreverbull/pkg/backtest/stream"
	"github.com/lhjnilsson/foreverbull/pkg/finance"
	"github.com/lhjnilsson/foreverbull/pkg/pb"
	"github.com/lhjnilsson/foreverbull/pkg/pb/backtest"
	financeServicePB "github.com/lhjnilsson/foreverbull/pkg/pb/finance/service"
	"github.com/stretchr/testify/mock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

// overCapCopy is the create sentence for a backtest over the download cap.
func overCapCopy(name, count string) string {
	return "Couldn't load data for backtest '" + name + "': it has " + count + " different symbols, and the limit is 10,000. " +
		"The backtest wasn't saved. Remove symbols or split them across several backtests, " +
		"then run `fbull backtest create` again."
}

// capAssets is n distinct asset symbols.
func capAssets(n int) []*backtest.Backtest_Period_Asset {
	assets := make([]*backtest.Backtest_Period_Asset, 0, n)
	for i := range n {
		assets = append(assets, &backtest.Backtest_Period_Asset{Symbol: fmt.Sprintf("S%05d", i), Enabled: true})
	}
	return assets
}

// createFromFile runs CreateBacktestFromBacktest for a file backtest with the
// SPY benchmark and assets in its one period.
func (test *BacktestCommandTest) createFromFile(name string, assets []*backtest.Backtest_Period_Asset) error {
	message := new(stream.MockMessage)
	message.On("ParsePayload", &bs.CreateBacktestFromBacktestCommand{}).Return(nil).Run(func(args mock.Arguments) {
		payload := args.Get(0).(*bs.CreateBacktestFromBacktestCommand)
		bytes, err := proto.Marshal(&backtest.Backtest{
			Name:      name,
			Benchmark: "SPY",
			Periods: []*backtest.Backtest_Period{{
				StartDate: &pb.Date{Year: 2022, Month: 1, Day: 1},
				EndDate:   &pb.Date{Year: 2022, Month: 12, Day: 31},
				Assets:    assets,
			}},
		})
		test.Require().NoError(err)
		payload.Backtest = bytes
	})
	message.On("MustGet", stream.DBDep).Return(test.db).Maybe()
	return command.CreateBacktestFromBacktest(context.TODO(), message)
}

func (test *BacktestCommandTest) requireNotStored(name string) {
	backtests := repository.Backtest{Conn: test.db}
	_, err := backtests.Get(context.TODO(), name)
	test.Require().ErrorIs(err, pgx.ErrNoRows)
	var periods int
	test.Require().NoError(test.db.QueryRow(context.TODO(),
		`SELECT count(*) FROM backtest_period WHERE backtest_name=$1`, name).Scan(&periods))
	test.Zero(periods)
}

func (test *BacktestCommandTest) TestSymbolCapIsTheFinanceConstant() {
	test.Equal(10_000, finance.MaxDownloadSymbols)
}

// SPY plus 10,000 assets is one over the cap: refused with the count and the
// limit, and nothing is stored, so no delete is needed.
func (test *BacktestCommandTest) TestCreateFromFileRefusesOneOverTheSymbolCap() {
	err := test.createFromFile("too-big", capAssets(finance.MaxDownloadSymbols))
	test.Require().EqualError(err, overCapCopy("too-big", "10,001"))
	test.requireNotStored("too-big")
}

// SPY plus 9,999 assets is exactly the cap and is stored; SPY listed as an
// asset as well as the benchmark doesn't count twice.
func (test *BacktestCommandTest) TestCreateFromFileAllowsExactlyTheSymbolCap() {
	assets := capAssets(finance.MaxDownloadSymbols - 1)
	assets = append(assets, &backtest.Backtest_Period_Asset{Symbol: "SPY", Enabled: true})
	test.Require().NoError(test.createFromFile("at-cap", assets))
	backtests := repository.Backtest{Conn: test.db}
	_, err := backtests.Get(context.TODO(), "at-cap")
	test.Require().NoError(err)
}

// Counts get thousands separators past 10,000 as well.
func (test *BacktestCommandTest) TestCreateFromFileOverCapCountHasSeparators() {
	err := test.createFromFile("much-too-big", capAssets(12_344))
	test.Require().EqualError(err, overCapCopy("much-too-big", "12,345"))
}

// An ETF create counts the benchmark and every holding's ticker before it
// asks the provider about any of them or stores anything.
func (test *BacktestCommandTest) TestCreateFromETFRefusesOverTheSymbolCap() {
	const etf = "iShares S&P 100"
	start, end := test.date("2024-01-02"), test.date("2024-03-28")
	lookback := time.Date(start.Year(), start.Month()-3, 1, 0, 0, 0, 0, time.UTC)
	holdings := make([]dependency.Asset, 0, finance.MaxDownloadSymbols)
	for i := range finance.MaxDownloadSymbols {
		holdings = append(holdings, dependency.Asset{
			Symbol: fmt.Sprintf("S%05d", i), Name: "Company", AssetType: "Equity", ISIN: fmt.Sprintf("US%010d", i),
		})
	}
	scraper := new(dependency.MockETFScraper)
	scraper.On("ListSnapshotDates", etf, start, end).Return([]time.Time{}, nil)
	scraper.On("ListSnapshotDates", etf, lookback, start).Return([]time.Time{start}, nil)
	scraper.On("GetSnapshot", etf, start).Return(holdings, nil)
	scraper.On("GetBenchmark", etf).Return("OEF", nil)
	mdclient := new(financeServicePB.MockMarketdataClient)

	message := etfCreateMessage(test.T(), "etf-too-big", etf, "2024-01-02", "2024-03-28")
	message.On("MustGet", stream.DBDep).Return(test.db).Maybe()
	message.On("MustGet", dependency.GetMarketdataClient).Return(mdclient).Maybe()
	message.On("MustGet", dependency.GetETFScraper).Return(scraper)

	err := command.CreateBacktestFromETF(context.TODO(), message)
	test.Require().EqualError(err, overCapCopy("etf-too-big", "10,001"))
	mdclient.AssertNotCalled(test.T(), "GetAsset", mock.Anything, mock.Anything)
	test.requireNotStored("etf-too-big")
}

// The download step counts the request it would send too, for a stored
// universe that grew past the cap after create: it removes the backtest and
// sends nothing.
func (test *BacktestCommandTest) TestDownloadFinancialDataRefusesOverTheSymbolCap() {
	backtests := repository.Backtest{Conn: test.db}
	test.Require().NoError(backtests.UpdatePeriod(context.TODO(), "test-backtest", &backtest.Backtest_Period{
		StartDate: &pb.Date{Year: 2022, Month: 1, Day: 1},
		EndDate:   &pb.Date{Year: 2022, Month: 12, Day: 31},
		Assets:    capAssets(finance.MaxDownloadSymbols),
	}))

	err := command.DownloadFinancialData(context.TODO(), test.downloadMessage("test-backtest"))
	test.Require().EqualError(err, overCapCopy("test-backtest", "10,001"))
	test.mdclient.AssertNotCalled(test.T(), "DownloadHistoricalData", mock.Anything, mock.Anything)
	test.requireNotStored("test-backtest")
}

// If the finance service refuses a create's download over the cap anyway, the
// create shows its own sentence with the broker's count, not the raw text.
func (test *BacktestCommandTest) TestDownloadFinancialDataShowsFinanceCapRefusalWithBrokerCount() {
	backtests := repository.Backtest{Conn: test.db}
	test.Require().NoError(backtests.UpdatePeriod(context.TODO(), "test-backtest", &backtest.Backtest_Period{
		StartDate: &pb.Date{Year: 2022, Month: 1, Day: 1},
		EndDate:   &pb.Date{Year: 2022, Month: 12, Day: 31},
		Assets:    capAssets(6_213),
	}))
	test.mdclient.On("DownloadHistoricalData", mock.Anything, mock.Anything).Return(nil,
		status.Error(codes.InvalidArgument, "one download can carry at most 10,000 distinct symbols, and this one has more. "+
			"Split it into smaller downloads."))

	err := command.DownloadFinancialData(context.TODO(), test.downloadMessage("test-backtest"))
	test.Require().EqualError(err, overCapCopy("test-backtest", "6,214"))
	test.NotContains(err.Error(), "Run the same create again")
	test.requireNotStored("test-backtest")
}
