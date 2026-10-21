package command_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/internal/stream"
	"github.com/lhjnilsson/foreverbull/internal/test_helper"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/repository"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/stream/command"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/stream/dependency"
	bs "github.com/lhjnilsson/foreverbull/pkg/backtest/stream"
	"github.com/lhjnilsson/foreverbull/pkg/finance/supplier"
	"github.com/lhjnilsson/foreverbull/pkg/pb"
	"github.com/lhjnilsson/foreverbull/pkg/pb/backtest"
	"github.com/lhjnilsson/foreverbull/pkg/pb/finance"
	financeServicePB "github.com/lhjnilsson/foreverbull/pkg/pb/finance/service"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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
	// The first published list opens at the backtest start, not at its as-of.
	test.Equal([]repository.MemberInterval{
		{ISIN: aaplISIN, FromDate: start, ToDate: test.toPtr("2023-02-28")},
		{ISIN: metaISIN, FromDate: start, ToDate: nil},
		{ISIN: msftISIN, FromDate: s1, ToDate: nil},
	}, listed)

	at, err := members.At(ctx, "test-backtest-2", start)
	test.Require().NoError(err)
	test.Equal([]string{aaplISIN, metaISIN}, at)
	at, err = members.At(ctx, "test-backtest-2", s0)
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
		{Symbol: "FB", FromDate: start, ToDate: test.toPtr("2023-01-31")},
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
		Provider:  supplier.DefaultProvider,
	}).Return(clientStreamMock, nil)

	err = command.DownloadFinancialData(ctx, download)
	test.Require().NoError(err)
	test.mdclient.AssertExpectations(test.T())
}

// An ETF universe is never empty on a day of the backtest. iShares lists are
// sampled at month-ends, so the days between the start and the first as-of
// hold the first published list. A rename (FB then META) and a drop (AAPL)
// later in the window do not leave a day without members.
func (test *BacktestCommandTest) TestCreateBacktestFromETFMembersFromStart() {
	aapl := dependency.Asset{Symbol: "AAPL", Name: "Apple Inc.", AssetType: "Equity", ISIN: aaplISIN}
	fb := dependency.Asset{Symbol: "FB", Name: "Meta Platforms Inc.", AssetType: "Equity", ISIN: metaISIN}
	meta := dependency.Asset{Symbol: "META", Name: "Meta Platforms Inc.", AssetType: "Equity", ISIN: metaISIN}
	msft := dependency.Asset{Symbol: "MSFT", Name: "Microsoft Corp.", AssetType: "Equity", ISIN: msftISIN}
	jan := test.date("2020-01-31")
	feb := test.date("2020-02-29")
	mar := test.date("2020-03-31")
	listings := map[time.Time][]dependency.Asset{
		jan: {aapl, fb},
		feb: {aapl, meta, msft},
		mar: {meta, msft},
	}

	cases := []struct {
		name  string
		start string
	}{
		{name: "etf-from-year-start", start: "2020-01-01"},
		{name: "etf-from-mid-month", start: "2020-01-15"},
		{name: "etf-from-month-end", start: "2020-01-31"},
	}
	for _, tc := range cases {
		test.Run(tc.name, func() {
			ctx := context.TODO()
			start := test.date(tc.start)
			end := mar
			scraper := new(dependency.MockETFScraper)
			scraper.On("ListSnapshotDates", "iShares S&P 100", start, end).Return([]time.Time{jan, feb, mar}, nil)
			for asOf, assets := range listings {
				scraper.On("GetSnapshot", "iShares S&P 100", asOf).Return(assets, nil)
			}
			scraper.On("GetBenchmark", "iShares S&P 100").Return("OEF", nil)
			mdclient := new(financeServicePB.MockMarketdataClient)
			test.mdclient = mdclient
			for _, symbol := range []string{"AAPL", "FB", "META", "MSFT"} {
				test.mockGetAsset(symbol, nil)
			}
			message := etfCreateMessage(test.T(), tc.name, "iShares S&P 100", tc.start, "2020-03-31")
			message.On("MustGet", stream.DBDep).Return(test.db)
			message.On("MustGet", dependency.GetMarketdataClient).Return(mdclient)
			message.On("MustGet", dependency.GetETFScraper).Return(scraper)

			test.Require().NoError(command.CreateBacktestFromETF(ctx, message))

			members := repository.UniverseMember{Conn: test.db}
			first := []string{aaplISIN, metaISIN}
			for _, day := range []time.Time{start, start.AddDate(0, 0, 1)} {
				at, err := members.At(ctx, tc.name, day)
				test.Require().NoError(err)
				test.Equal(first, at, "members on %s are the first published list", day.Format("2006-01-02"))
			}
			symbols, err := members.SymbolsAt(ctx, tc.name, start)
			test.Require().NoError(err)
			test.Equal([]string{"AAPL", "FB"}, symbols)

			// The window is one quarter, so the walk is bounded by end.
			empty := make([]string, 0)
			for day := start; !day.After(end); day = day.AddDate(0, 0, 1) {
				at, err := members.At(ctx, tc.name, day)
				test.Require().NoError(err)
				if len(at) == 0 {
					empty = append(empty, day.Format("2006-01-02"))
				}
			}
			test.Empty(empty, "days with no members")
		})
	}
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
		Provider:  supplier.DefaultProvider,
	}).Return(clientStreamMock, nil)

	err = command.DownloadFinancialData(context.TODO(), msg)
	test.Require().NoError(err)
	test.mdclient.AssertExpectations(test.T())
}

func (test *BacktestCommandTest) TestDownloadFinancialDataStopsOnBlockedProvider() {
	const blocked = "Stooq blocked the download for AAPL (it returned a bot-check page instead of data). Retry with --provider yahoo."
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
	clientStreamMock.On("Recv", mock.Anything).Return(nil, status.Error(codes.FailedPrecondition, blocked))
	test.mdclient.On("DownloadHistoricalData", mock.Anything, mock.Anything).Return(clientStreamMock, nil)

	err = command.DownloadFinancialData(context.TODO(), msg)
	test.Require().EqualError(err, blocked)

	// The CREATING step stored the backtest; a block deletes it again so the
	// retry with the other provider is not refused as a duplicate name.
	_, err = backtests.Get(context.TODO(), "test-backtest")
	test.Require().ErrorIs(err, pgx.ErrNoRows)

	retry := new(stream.MockMessage)
	retry.On("ParsePayload", &bs.CreateBacktestFromBacktestCommand{}).Return(nil).Run(func(args mock.Arguments) {
		payload := args.Get(0).(*bs.CreateBacktestFromBacktestCommand)
		bytes, err := proto.Marshal(test.backtest)
		test.Require().NoError(err)
		payload.Backtest = bytes
	})
	retry.On("MustGet", stream.DBDep).Return(test.db)
	test.Require().NoError(command.CreateBacktestFromBacktest(context.TODO(), retry))

	b, err := backtests.Get(context.TODO(), "test-backtest")
	test.Require().NoError(err)
	test.Equal("test-backtest", b.Name)
}

func (test *BacktestCommandTest) downloadMessage(name string) *stream.MockMessage {
	msg := new(stream.MockMessage)
	msg.On("MustGet", stream.DBDep).Return(test.db)
	msg.On("MustGet", dependency.GetMarketdataClient).Return(test.mdclient)
	msg.On("ParsePayload", &bs.DownloadFinancialDataCommand{}).Return(nil).Run(func(args mock.Arguments) {
		payload := args.Get(0).(*bs.DownloadFinancialDataCommand)
		payload.BacktestName = name
	})
	return msg
}

func (test *BacktestCommandTest) createAgain() {
	retry := new(stream.MockMessage)
	retry.On("ParsePayload", &bs.CreateBacktestFromBacktestCommand{}).Return(nil).Run(func(args mock.Arguments) {
		payload := args.Get(0).(*bs.CreateBacktestFromBacktestCommand)
		bytes, err := proto.Marshal(test.backtest)
		test.Require().NoError(err)
		payload.Backtest = bytes
	})
	retry.On("MustGet", stream.DBDep).Return(test.db)
	test.Require().NoError(command.CreateBacktestFromBacktest(context.TODO(), retry))
}

func (test *BacktestCommandTest) TestDownloadFinancialDataStreamErrorRemovesBacktest() {
	backtests := repository.Backtest{Conn: test.db}
	err := backtests.UpdatePeriod(context.TODO(), "test-backtest", &backtest.Backtest_Period{
		StartDate: &pb.Date{Year: 2022, Month: 1, Day: 1},
		EndDate:   &pb.Date{Year: 2022, Month: 12, Day: 31},
		Assets: []*backtest.Backtest_Period_Asset{
			{Symbol: "AAPL", Enabled: true},
			{Symbol: "MSFT", Enabled: true},
		},
	})
	test.Require().NoError(err)

	// MSFT fails on its own first, then the stream breaks. We can't tell
	// which bars were stored, so the whole backtest goes.
	clientStreamMock := new(financeServicePB.MockMarketdata_DownloadHistoricalDataClient[interface{}])
	clientStreamMock.On("Recv", mock.Anything).Return(&financeServicePB.DownloadHistoricalDataResponse{
		Symbol: "MSFT",
		Status: financeServicePB.DownloadHistoricalDataResponse_STATUS_ERROR,
	}, nil).Once()
	clientStreamMock.On("Recv", mock.Anything).Return(nil, status.Error(codes.Unavailable, "connection refused"))
	test.mdclient.On("DownloadHistoricalData", mock.Anything, mock.Anything).Return(clientStreamMock, nil)

	err = command.DownloadFinancialData(context.TODO(), test.downloadMessage("test-backtest"))
	test.Require().EqualError(err, "Downloading prices failed: connection refused. Nothing was changed. Run the same create again.")

	_, err = backtests.Get(context.TODO(), "test-backtest")
	test.Require().ErrorIs(err, pgx.ErrNoRows)
	var periods, assets int
	test.Require().NoError(test.db.QueryRow(context.TODO(),
		`SELECT count(*) FROM backtest_period WHERE backtest_name=$1`, "test-backtest").Scan(&periods))
	test.Require().NoError(test.db.QueryRow(context.TODO(),
		`SELECT count(*) FROM backtest_period_asset WHERE backtest_name=$1`, "test-backtest").Scan(&assets))
	test.Zero(periods)
	test.Zero(assets)

	// Create again under the same name.
	test.createAgain()
	b, err := backtests.Get(context.TODO(), "test-backtest")
	test.Require().NoError(err)
	test.Equal("test-backtest", b.Name)
}

func (test *BacktestCommandTest) TestDownloadFinancialDataConnectionErrorRemovesBacktest() {
	test.mdclient.On("DownloadHistoricalData", mock.Anything, mock.Anything).
		Return(nil, status.Error(codes.Unavailable, "Finance service unreachable"))

	err := command.DownloadFinancialData(context.TODO(), test.downloadMessage("test-backtest"))
	test.Require().EqualError(err, "Downloading prices failed: finance service unreachable. Nothing was changed. Run the same create again.")

	backtests := repository.Backtest{Conn: test.db}
	_, err = backtests.Get(context.TODO(), "test-backtest")
	test.Require().ErrorIs(err, pgx.ErrNoRows)

	test.createAgain()
	_, err = backtests.Get(context.TODO(), "test-backtest")
	test.Require().NoError(err)
}

func (test *BacktestCommandTest) TestDownloadFinancialDataKeepsBacktestWhenSomeSymbolsFail() {
	backtests := repository.Backtest{Conn: test.db}
	err := backtests.UpdatePeriod(context.TODO(), "test-backtest", &backtest.Backtest_Period{
		StartDate: &pb.Date{Year: 2022, Month: 1, Day: 1},
		EndDate:   &pb.Date{Year: 2022, Month: 12, Day: 31},
		Assets: []*backtest.Backtest_Period_Asset{
			{Symbol: "AAPL", Enabled: true},
		},
	})
	test.Require().NoError(err)

	clientStreamMock := new(financeServicePB.MockMarketdata_DownloadHistoricalDataClient[interface{}])
	clientStreamMock.On("Recv", mock.Anything).Return(&financeServicePB.DownloadHistoricalDataResponse{
		Symbol: "AAPL",
		Status: financeServicePB.DownloadHistoricalDataResponse_STATUS_ERROR,
	}, nil).Once()
	clientStreamMock.On("Recv", mock.Anything).Return(nil, io.EOF)
	test.mdclient.On("DownloadHistoricalData", mock.Anything, mock.Anything).Return(clientStreamMock, nil)

	test.Require().NoError(command.DownloadFinancialData(context.TODO(), test.downloadMessage("test-backtest")))

	b, err := backtests.Get(context.TODO(), "test-backtest")
	test.Require().NoError(err)
	for _, period := range b.Periods {
		for _, asset := range period.Assets {
			if asset.Symbol == "AAPL" {
				test.False(asset.Enabled)
			}
		}
	}
}

func civilToday() string {
	year, month, day := time.Now().Date()
	return fmt.Sprintf("%04d-%02d-%02d", year, int(month), day)
}

func etfCreateMessage(t *testing.T, name, etf, start, end string) *stream.MockMessage {
	t.Helper()
	message := new(stream.MockMessage)
	message.On("ParsePayload", &bs.CreateBacktestFromETFCommand{}).Return(nil).Run(func(args mock.Arguments) {
		payload := args.Get(0).(*bs.CreateBacktestFromETFCommand)
		payload.BacktestName = name
		payload.ETFName = etf
		payload.StartDate = start
		payload.EndDate = end
	})
	return message
}

func TestCreateBacktestFromETFRejectsInvalidInput(t *testing.T) {
	today := civilToday()
	cases := []struct {
		name  string
		etf   string
		start string
		end   string
		want  string
	}{
		{"", "iShares S&P 100", "2020-01-01", "2020-12-31", "Enter a name for the backtest."},
		{"   ", "iShares S&P 100", "2020-01-01", "2020-12-31", "Enter a name for the backtest."},
		{"demo", "not-a-fund", "2020-01-01", "2020-12-31", "Unknown ETF 'not-a-fund'. Known funds: iShares S&P 100."},
		{"demo", "iShares S&P 100", "not-a-date", "2020-12-31", "'not-a-date' is not a date. Use YYYY-MM-DD, for example 2024-01-31."},
		{"demo", "iShares S&P 100", "20200101", "2020-12-31", "'20200101' is not a date. Use YYYY-MM-DD, for example 2024-01-31."},
		{"demo", "iShares S&P 100", "2020-02-31", "2020-03-31", "'2020-02-31' is not a date. Use YYYY-MM-DD, for example 2024-01-31."},
		{"demo", "iShares S&P 100", "1999-01-01", "2020-12-31", "iShares S&P 100 started on 2000-10-23. Choose a start date on or after that day."},
		{"demo", "iShares S&P 100", "2999-01-01", "2999-12-31", "2999-01-01 is after today (" + today + ")."},
		{"demo", "iShares S&P 100", "2020-06-01", "2020-01-01", "End date 2020-01-01 is before start date 2020-06-01."},
		{"demo", "iShares S&P 100", "2020-01-01", "not-a-date", "'not-a-date' is not a date. Use YYYY-MM-DD, for example 2024-01-31."},
		{"demo", "iShares S&P 100", "2020-01-01", "2999-01-01", "End date 2999-01-01 is after today (" + today + ")."},
	}
	for _, tc := range cases {
		t.Run(tc.want, func(t *testing.T) {
			message := etfCreateMessage(t, tc.name, tc.etf, tc.start, tc.end)
			err := command.CreateBacktestFromETF(context.Background(), message)
			require.EqualError(t, err, tc.want)
			require.NotContains(t, err.Error(), "json")
			require.NotContains(t, err.Error(), "parsing time")
			message.AssertNumberOfCalls(t, "MustGet", 0)
		})
	}
}

func TestCreateBacktestFromETFLongRangeIsNotInvalid(t *testing.T) {
	message := etfCreateMessage(t, "sp100", "iShares S&P 100", "2020-01-01", "2025-12-31")
	scraper := new(dependency.MockETFScraper)
	message.On("MustGet", dependency.GetETFScraper).Return(scraper)
	start := time.Date(2020, time.January, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2025, time.December, 31, 0, 0, 0, 0, time.UTC)
	scraper.On("ListSnapshotDates", "iShares S&P 100", start, end).Return(nil, errors.New("sentinel"))

	err := command.CreateBacktestFromETF(context.Background(), message)
	require.ErrorContains(t, err, "sentinel")
	require.NotContains(t, err.Error(), "is not a date")
	require.NotContains(t, err.Error(), "after today")
	require.NotContains(t, err.Error(), "started on")
	require.NotContains(t, err.Error(), "Unknown ETF")
	message.AssertNotCalled(t, "MustGet", stream.DBDep)
	scraper.AssertExpectations(t)
}
