package command_test

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/internal/stream"
	"github.com/lhjnilsson/foreverbull/internal/test_helper"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/repository"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/stream/command"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/stream/dependency"
	bs "github.com/lhjnilsson/foreverbull/pkg/backtest/stream"
	"github.com/lhjnilsson/foreverbull/pkg/pb"
	"github.com/lhjnilsson/foreverbull/pkg/pb/backtest"
	"github.com/lhjnilsson/foreverbull/pkg/pb/finance"
	financeServicePB "github.com/lhjnilsson/foreverbull/pkg/pb/finance/service"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"google.golang.org/protobuf/proto"
)

const (
	aaplISIN = "US0378331005"
	metaISIN = "US30303M1027"
	msftISIN = "US5949181045"
)

type BacktestCommandTest struct {
	suite.Suite

	db         *pgxpool.Pool
	etfscraper *dependency.MockETFScraper
	mdclient   *financeServicePB.MockMarketdataClient

	backtest *backtest.Backtest
}

func TestBacktest(t *testing.T) {
	suite.Run(t, new(BacktestCommandTest))
}

func (test *BacktestCommandTest) SetupSuite() {
	var err error
	test_helper.SetupEnvironment(test.T(), &test_helper.Containers{
		Postgres: true,
	})

	test.db, err = pgxpool.New(context.Background(), environment.GetPostgresURL())
	test.Require().NoError(err)
	err = repository.Recreate(context.Background(), test.db)
	test.Require().NoError(err)

	test.backtest = &backtest.Backtest{
		Name:      "test-backtest",
		Benchmark: "SPY",
		Periods: []*backtest.Backtest_Period{
			{
				StartDate: &pb.Date{Year: 2022, Month: 1, Day: 1},
				EndDate:   &pb.Date{Year: 2022, Month: 12, Day: 31},
			},
			{
				StartDate: &pb.Date{Year: 2023, Month: 1, Day: 1},
				EndDate:   &pb.Date{Year: 2023, Month: 12, Day: 31},
			},
		},
	}
	backtests := repository.Backtest{Conn: test.db}
	_, err = backtests.Create(context.TODO(), test.backtest)
	test.Require().NoError(err)
}

func (test *BacktestCommandTest) TearDownSuite() {
}

func (test *BacktestCommandTest) SetupTest() {
	err := repository.Recreate(context.Background(), test.db)
	test.Require().NoError(err)
	backtests := repository.Backtest{Conn: test.db}
	_, err = backtests.Create(context.TODO(), test.backtest)
	test.Require().NoError(err)

	test.etfscraper = new(dependency.MockETFScraper)
	test.mdclient = new(financeServicePB.MockMarketdataClient)
}

func (test *BacktestCommandTest) date(s string) time.Time {
	d, err := time.Parse("2006-01-02", s)
	test.Require().NoError(err)
	return d
}

func (test *BacktestCommandTest) toPtr(s string) *time.Time {
	d := test.date(s)
	return &d
}

func (test *BacktestCommandTest) mockGetAsset(symbol string, err error) {
	call := test.mdclient.On("GetAsset", mock.Anything, &financeServicePB.GetAssetRequest{
		Identifier: &financeServicePB.GetAssetRequest_Symbol{Symbol: symbol},
	})
	if err != nil {
		call.Return(nil, err)
		return
	}
	call.Return(&financeServicePB.GetAssetResponse{
		Asset: &finance.Asset{Symbol: symbol},
	}, nil)
}

func (test *BacktestCommandTest) TearDownTest() {
}

func (test *BacktestCommandTest) TestCreateBacktestFromBacktest() {
	message := new(stream.MockMessage)
	message.On("ParsePayload", &bs.CreateBacktestFromBacktestCommand{}).Return(nil).Run(func(args mock.Arguments) {
		payload := args.Get(0).(*bs.CreateBacktestFromBacktestCommand)
		b := backtest.Backtest{
			Name:      "test-backtest-from_backtest",
			Benchmark: "SPY",
			Periods: []*backtest.Backtest_Period{
				{
					StartDate: &pb.Date{Year: 2022, Month: 1, Day: 1},
					EndDate:   &pb.Date{Year: 2022, Month: 12, Day: 31},
				},
				{
					StartDate: &pb.Date{Year: 2023, Month: 1, Day: 1},
					EndDate:   &pb.Date{Year: 2023, Month: 12, Day: 31},
				},
			},
		}
		bytes, err := proto.Marshal(&b)
		test.Require().NoError(err)
		payload.Backtest = bytes
	})
	message.On("MustGet", stream.DBDep).Return(test.db)

	err := command.CreateBacktestFromBacktest(context.TODO(), message)
	test.Require().NoError(err)

	backtests := repository.Backtest{Conn: test.db}
	backtest, err := backtests.Get(context.TODO(), "test-backtest-from_backtest")
	test.Require().NoError(err)
	test.Require().NotNil(backtest)
}

func (test *BacktestCommandTest) TestCreateBacktestFromETF() {
	ctx := context.TODO()
	message := new(stream.MockMessage)
	message.On("ParsePayload", &bs.CreateBacktestFromETFCommand{}).Return(nil).Run(func(args mock.Arguments) {
		payload := args.Get(0).(*bs.CreateBacktestFromETFCommand)
		payload.BacktestName = "test-backtest-2"
		payload.ETFName = "iShares S&P 100"
		payload.StartDate = "2022-01-01"
		payload.EndDate = "2023-12-31"
	})
	message.On("MustGet", stream.DBDep).Return(test.db)
	message.On("MustGet", dependency.GetMarketdataClient).Return(test.mdclient)
	message.On("MustGet", dependency.GetETFScraper).Return(test.etfscraper)

	s0 := test.date("2022-12-31")
	s1 := test.date("2023-01-31")
	s2 := test.date("2023-02-28")
	start := test.date("2022-01-01")
	end := test.date("2023-12-31")

	aapl := dependency.Asset{Symbol: "AAPL", Name: "Apple Inc.", Sector: "Technology", AssetType: "Equity", ISIN: aaplISIN}
	fb := dependency.Asset{Symbol: "FB", Name: "Meta Platforms Inc.", Sector: "Communication", AssetType: "Equity", ISIN: metaISIN}
	meta := dependency.Asset{Symbol: "META", Name: "Meta Platforms Inc.", Sector: "Communication", AssetType: "Equity", ISIN: metaISIN}
	msft := dependency.Asset{Symbol: "MSFT", Name: "Microsoft Corp.", Sector: "Technology", AssetType: "Equity", ISIN: msftISIN}
	cash := dependency.Asset{Symbol: "CASH_USD", Name: "USD Cash", Sector: "-", AssetType: "Cash", ISIN: ""}

	test.etfscraper.On("ListSnapshotDates", "iShares S&P 100", start, end).Return([]time.Time{s0, s1, s2}, nil)
	test.etfscraper.On("GetSnapshot", "iShares S&P 100", s0).Return([]dependency.Asset{aapl, fb, cash}, nil)
	test.etfscraper.On("GetSnapshot", "iShares S&P 100", s1).Return([]dependency.Asset{aapl, meta, msft, cash}, nil)
	test.etfscraper.On("GetSnapshot", "iShares S&P 100", s2).Return([]dependency.Asset{meta, msft, cash}, nil)
	test.etfscraper.On("GetBenchmark", "iShares S&P 100").Return("OEF", nil)

	test.mockGetAsset("AAPL", nil)
	test.mockGetAsset("FB", errors.New("unknown ticker"))
	test.mockGetAsset("META", nil)
	test.mockGetAsset("MSFT", nil)

	err := command.CreateBacktestFromETF(ctx, message)
	test.Require().NoError(err)
	test.etfscraper.AssertNotCalled(test.T(), "GetAssets")
	test.etfscraper.AssertExpectations(test.T())
	test.mdclient.AssertExpectations(test.T())

	backtests := repository.Backtest{Conn: test.db}
	stored, err := backtests.Get(ctx, "test-backtest-2")
	test.Require().NoError(err)
	test.Equal("OEF", stored.Benchmark)
	test.Len(stored.Periods, 2)
	test.Equal(&pb.Date{Year: 2022, Month: 1, Day: 1}, stored.Periods[0].StartDate)
	test.Equal(&pb.Date{Year: 2022, Month: 12, Day: 31}, stored.Periods[0].EndDate)
	test.Equal([]*backtest.Backtest_Period_Asset{
		{Symbol: "FB", Enabled: false},
		{Symbol: "AAPL", Enabled: true},
	}, stored.Periods[0].Assets)
	test.Equal([]*backtest.Backtest_Period_Asset{
		{Symbol: "MSFT", Enabled: true},
		{Symbol: "META", Enabled: true},
		{Symbol: "FB", Enabled: false},
		{Symbol: "AAPL", Enabled: true},
	}, stored.Periods[1].Assets)

	members := repository.UniverseMember{Conn: test.db}
	listed, err := members.List(ctx, "test-backtest-2")
	test.Require().NoError(err)
	test.Equal([]repository.MemberInterval{
		{ISIN: aaplISIN, FromDate: s0, ToDate: test.toPtr("2023-02-28")},
		{ISIN: metaISIN, FromDate: s0, ToDate: nil},
		{ISIN: msftISIN, FromDate: s1, ToDate: nil},
	}, listed)

	at, err := members.At(ctx, "test-backtest-2", s0)
	test.Require().NoError(err)
	test.Equal([]string{aaplISIN, metaISIN}, at)
	at, err = members.At(ctx, "test-backtest-2", s1)
	test.Require().NoError(err)
	test.Equal([]string{aaplISIN, metaISIN, msftISIN}, at)

	var instID, instISIN string
	err = test.db.QueryRow(ctx, `SELECT id, isin FROM instrument WHERE isin=$1`, metaISIN).Scan(&instID, &instISIN)
	test.Require().NoError(err)
	test.Equal(metaISIN, instID)
	test.Equal(metaISIN, instISIN)

	var n int64
	err = test.db.QueryRow(ctx, `SELECT COUNT(*) FROM instrument WHERE isin=$1`, metaISIN).Scan(&n)
	test.Require().NoError(err)
	test.Equal(int64(1), n)

	type aliasRow struct {
		Symbol   string
		FromDate time.Time
		ToDate   *time.Time
	}
	rows, err := test.db.Query(ctx, `
		SELECT symbol, from_date, to_date FROM asset_symbol
		 WHERE isin=$1 ORDER BY from_date, symbol`, metaISIN)
	test.Require().NoError(err)
	defer rows.Close()
	got := make([]aliasRow, 0)
	for rows.Next() {
		var row aliasRow
		test.Require().NoError(rows.Scan(&row.Symbol, &row.FromDate, &row.ToDate))
		row.FromDate = time.Date(row.FromDate.Year(), row.FromDate.Month(), row.FromDate.Day(), 0, 0, 0, 0, time.UTC)
		if row.ToDate != nil {
			to := time.Date(row.ToDate.Year(), row.ToDate.Month(), row.ToDate.Day(), 0, 0, 0, 0, time.UTC)
			row.ToDate = &to
		}
		got = append(got, row)
	}
	test.Require().NoError(rows.Err())
	test.Equal([]aliasRow{
		{Symbol: "FB", FromDate: s0, ToDate: test.toPtr("2023-01-31")},
		{Symbol: "META", FromDate: s1, ToDate: nil},
	}, got)

	var resolvedFB, resolvedMeta string
	err = test.db.QueryRow(ctx, `
		SELECT i.id FROM asset_symbol s
		  JOIN instrument i ON i.isin = s.isin
		 WHERE s.symbol=$1 AND s.from_date <= $2::date
		   AND (s.to_date IS NULL OR $2::date < s.to_date)`,
		"FB", s0.Format("2006-01-02")).Scan(&resolvedFB)
	test.Require().NoError(err)
	err = test.db.QueryRow(ctx, `
		SELECT i.id FROM asset_symbol s
		  JOIN instrument i ON i.isin = s.isin
		 WHERE s.symbol=$1 AND s.from_date <= $2::date
		   AND (s.to_date IS NULL OR $2::date < s.to_date)`,
		"META", s1.Format("2006-01-02")).Scan(&resolvedMeta)
	test.Require().NoError(err)
	test.Equal(instID, resolvedFB)
	test.Equal(instID, resolvedMeta)

	download := new(stream.MockMessage)
	download.On("MustGet", stream.DBDep).Return(test.db)
	download.On("MustGet", dependency.GetMarketdataClient).Return(test.mdclient)
	download.On("ParsePayload", &bs.DownloadFinancialDataCommand{}).Return(nil).Run(func(args mock.Arguments) {
		payload := args.Get(0).(*bs.DownloadFinancialDataCommand)
		payload.BacktestName = "test-backtest-2"
	})
	clientStreamMock := new(financeServicePB.MockMarketdata_DownloadHistoricalDataClient[interface{}])
	clientStreamMock.On("Recv", mock.Anything).Return(nil, io.EOF)
	test.mdclient.On("DownloadHistoricalData", mock.Anything, &financeServicePB.DownloadHistoricalDataRequest{
		Symbols:   []string{"OEF", "AAPL", "FB", "META", "MSFT"},
		StartDate: &pb.Date{Year: 2022, Month: 1, Day: 1},
		EndDate:   &pb.Date{Year: 2023, Month: 12, Day: 31},
	}).Return(clientStreamMock, nil)

	err = command.DownloadFinancialData(ctx, download)
	test.Require().NoError(err)
	test.mdclient.AssertExpectations(test.T())
}

func (test *BacktestCommandTest) TestDownloadFinancialData() {
	backtests := repository.Backtest{Conn: test.db}
	err := backtests.UpdatePeriod(context.TODO(), "test-backtest", &backtest.Backtest_Period{
		StartDate: &pb.Date{Year: 2022, Month: 1, Day: 1},
		EndDate:   &pb.Date{Year: 2022, Month: 12, Day: 31},
		Assets: []*backtest.Backtest_Period_Asset{
			{
				Symbol:  "AAPL",
				Enabled: true,
			},
		},
	})
	test.Require().NoError(err)
	msg := new(stream.MockMessage)
	msg.On("MustGet", stream.DBDep).Return(test.db)
	msg.On("MustGet", dependency.GetMarketdataClient).Return(test.mdclient)
	msg.On("ParsePayload", &bs.DownloadFinancialDataCommand{}).Return(nil).Run(func(args mock.Arguments) {
		payload := args.Get(0).(*bs.DownloadFinancialDataCommand)
		payload.BacktestName = "test-backtest"
	})

	clientStreamMock := new(financeServicePB.MockMarketdata_DownloadHistoricalDataClient[interface{}])
	clientStreamMock.On("Recv", mock.Anything).Return(nil, io.EOF)

	test.mdclient.On("DownloadHistoricalData", mock.Anything, &financeServicePB.DownloadHistoricalDataRequest{
		Symbols:   []string{"SPY", "AAPL"},
		StartDate: &pb.Date{Year: 2022, Month: 1, Day: 1},
		EndDate:   &pb.Date{Year: 2023, Month: 12, Day: 31},
	}).Return(clientStreamMock, nil)

	err = command.DownloadFinancialData(context.TODO(), msg)
	test.Require().NoError(err)
	test.mdclient.AssertExpectations(test.T())
}
