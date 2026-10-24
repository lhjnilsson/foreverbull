package command_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"slices"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/internal/storage"
	"github.com/lhjnilsson/foreverbull/internal/stream"
	"github.com/lhjnilsson/foreverbull/internal/test_helper"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/engine"
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

	// The seed is the last list published on or before the start.
	seed := test.date("2021-12-31")
	test.etfscraper.On("ListSnapshotDates", "iShares S&P 100", start, end).Return([]time.Time{s0, s1, s2}, nil)
	test.etfscraper.On("ListSnapshotDates", "iShares S&P 100", test.date("2021-10-01"), start).Return([]time.Time{seed}, nil)
	test.etfscraper.On("GetSnapshot", "iShares S&P 100", seed).Return([]dependency.Asset{aapl, fb, cash}, nil)
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
	// The 2021-12-31 list opens at the backtest start, not at its as-of.
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
// sampled at month-ends, so the days between the start and the next as-of
// hold the last list published on or before the start. MSFT first appears in
// the 2020-02-29 list and is not a member before it. A rename (FB then META)
// and a drop (AAPL) later in the window do not leave a day without members.
func (test *BacktestCommandTest) TestCreateBacktestFromETFMembersFromStart() {
	aapl := dependency.Asset{Symbol: "AAPL", Name: "Apple Inc.", AssetType: "Equity", ISIN: aaplISIN}
	fb := dependency.Asset{Symbol: "FB", Name: "Meta Platforms Inc.", AssetType: "Equity", ISIN: metaISIN}
	meta := dependency.Asset{Symbol: "META", Name: "Meta Platforms Inc.", AssetType: "Equity", ISIN: metaISIN}
	msft := dependency.Asset{Symbol: "MSFT", Name: "Microsoft Corp.", AssetType: "Equity", ISIN: msftISIN}
	dec := test.date("2019-12-31")
	jan := test.date("2020-01-31")
	feb := test.date("2020-02-29")
	mar := test.date("2020-03-31")
	listings := map[time.Time][]dependency.Asset{
		dec: {aapl, fb},
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
			before := []time.Time{dec}
			if !jan.After(start) {
				before = append(before, jan)
			}
			scraper.On("ListSnapshotDates", "iShares S&P 100", test.date("2019-10-01"), start).Return(before, nil)
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
			at, err := members.At(ctx, tc.name, feb.AddDate(0, 0, -1))
			test.Require().NoError(err)
			test.Equal(first, at, "MSFT is not a member before the list that adds it")
			at, err = members.At(ctx, tc.name, feb)
			test.Require().NoError(err)
			test.Equal([]string{aaplISIN, metaISIN, msftISIN}, at)

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

// createETFBacktest runs CreateBacktestFromETF for name over [start, end]
// against fixture lists keyed by as-of date.
func (test *BacktestCommandTest) createETFBacktest(name, start, end string, listings map[time.Time][]dependency.Asset) error {
	const etf = "iShares S&P 100"
	from, to := test.date(start), test.date(end)
	lookback := time.Date(from.Year(), from.Month()-3, 1, 0, 0, 0, 0, time.UTC)
	window := make([]time.Time, 0)
	before := make([]time.Time, 0)
	for asOf := range listings {
		if !asOf.Before(from) && !asOf.After(to) {
			window = append(window, asOf)
		}
		if !asOf.Before(lookback) && !asOf.After(from) {
			before = append(before, asOf)
		}
	}
	slices.SortFunc(window, func(a, b time.Time) int { return a.Compare(b) })
	slices.SortFunc(before, func(a, b time.Time) int { return a.Compare(b) })

	scraper := new(dependency.MockETFScraper)
	scraper.On("ListSnapshotDates", etf, from, to).Return(window, nil)
	scraper.On("ListSnapshotDates", etf, lookback, from).Return(before, nil)
	for asOf, assets := range listings {
		scraper.On("GetSnapshot", etf, asOf).Return(assets, nil)
	}
	scraper.On("GetBenchmark", etf).Return("OEF", nil)
	mdclient := new(financeServicePB.MockMarketdataClient)
	mdclient.On("GetAsset", mock.Anything, mock.Anything).Return(&financeServicePB.GetAssetResponse{
		Asset: &finance.Asset{Symbol: "ANY"},
	}, nil)

	message := etfCreateMessage(test.T(), name, etf, start, end)
	message.On("MustGet", stream.DBDep).Return(test.db)
	message.On("MustGet", dependency.GetMarketdataClient).Return(mdclient)
	message.On("MustGet", dependency.GetETFScraper).Return(scraper)
	return command.CreateBacktestFromETF(context.TODO(), message)
}

// renameListings are month-end S&P 100 lists where FB becomes META in the
// 2021-10-31 list. Same ISIN on both sides.
func (test *BacktestCommandTest) renameListings() map[time.Time][]dependency.Asset {
	aapl := dependency.Asset{Symbol: "AAPL", Name: "Apple Inc.", AssetType: "Equity", ISIN: aaplISIN}
	fb := dependency.Asset{Symbol: "FB", Name: "Facebook Inc.", AssetType: "Equity", ISIN: metaISIN}
	meta := dependency.Asset{Symbol: "META", Name: "Meta Platforms Inc.", AssetType: "Equity", ISIN: metaISIN}
	listings := map[time.Time][]dependency.Asset{}
	for _, asOf := range []string{"2021-05-31", "2021-06-30", "2021-07-31", "2021-08-31", "2021-09-30"} {
		listings[test.date(asOf)] = []dependency.Asset{aapl, fb}
	}
	for _, asOf := range []string{"2021-10-31", "2021-11-30", "2021-12-31"} {
		listings[test.date(asOf)] = []dependency.Asset{aapl, meta}
	}
	return listings
}

func (test *BacktestCommandTest) metaAliases() []repository.AliasInterval {
	instruments := repository.Instrument{Conn: test.db}
	aliases, err := instruments.ListAliases(context.TODO(), metaISIN)
	test.Require().NoError(err)
	return aliases
}

// asset_symbol is shared by every ETF backtest. One that ends before the
// rename leaves FB open-ended; a later one that covers the rename still
// creates, and the alias is FB before the rename and META from it.
func (test *BacktestCommandTest) TestCreateBacktestFromETFAfterBacktestThatEndsBeforeRename() {
	listings := test.renameListings()
	rename := test.date("2021-10-31")

	test.Require().NoError(test.createETFBacktest("before-rename", "2021-07-01", "2021-09-30", listings))
	test.Equal([]repository.AliasInterval{
		{ISIN: metaISIN, Symbol: "FB", FromDate: test.date("2021-07-01"), ToDate: nil},
	}, test.metaAliases())

	test.Require().NoError(test.createETFBacktest("over-rename", "2021-09-01", "2021-12-31", listings))
	test.Equal([]repository.AliasInterval{
		{ISIN: metaISIN, Symbol: "FB", FromDate: test.date("2021-07-01"), ToDate: &rename},
		{ISIN: metaISIN, Symbol: "META", FromDate: rename, ToDate: nil},
	}, test.metaAliases())

	members := repository.UniverseMember{Conn: test.db}
	for _, tc := range []struct {
		name string
		day  string
		want []string
	}{
		{name: "before-rename", day: "2021-09-30", want: []string{"AAPL", "FB"}},
		{name: "over-rename", day: "2021-10-30", want: []string{"AAPL", "FB"}},
		{name: "over-rename", day: "2021-10-31", want: []string{"AAPL", "META"}},
	} {
		symbols, err := members.SymbolsAt(context.TODO(), tc.name, test.date(tc.day))
		test.Require().NoError(err)
		test.Equal(tc.want, symbols, "%s on %s", tc.name, tc.day)
	}
}

// Reverse order: the backtest over the rename is created first. An older
// backtest that sees FB open-ended still creates, and its FB ends where META
// starts instead of reopening the alias.
func (test *BacktestCommandTest) TestCreateBacktestFromETFBeforeRenameAfterBacktestOverIt() {
	listings := test.renameListings()
	rename := test.date("2021-10-31")

	test.Require().NoError(test.createETFBacktest("over-rename", "2021-09-01", "2021-12-31", listings))
	test.Require().NoError(test.createETFBacktest("before-rename", "2021-07-01", "2021-09-30", listings))
	test.Equal([]repository.AliasInterval{
		{ISIN: metaISIN, Symbol: "FB", FromDate: test.date("2021-07-01"), ToDate: &rename},
		{ISIN: metaISIN, Symbol: "META", FromDate: rename, ToDate: nil},
	}, test.metaAliases())
}

// Published lists that name two tickers for one ISIN on the same day are a
// real conflict, and the second create is refused.
func (test *BacktestCommandTest) TestCreateBacktestFromETFRefusesConflictingTicker() {
	listings := test.renameListings()
	test.Require().NoError(test.createETFBacktest("over-rename", "2021-09-01", "2021-12-31", listings))

	other := dependency.Asset{Symbol: "FBX", Name: "Facebook Inc.", AssetType: "Equity", ISIN: metaISIN}
	aapl := dependency.Asset{Symbol: "AAPL", Name: "Apple Inc.", AssetType: "Equity", ISIN: aaplISIN}
	conflicting := map[time.Time][]dependency.Asset{
		test.date("2021-10-31"): {aapl, other},
		test.date("2021-11-30"): {aapl, other},
	}
	err := test.createETFBacktest("conflict", "2021-10-31", "2021-11-30", conflicting)
	test.Require().ErrorIs(err, repository.ErrAliasConflict)
	test.Require().ErrorContains(err, "asset_symbol_no_overlap")

	rename := test.date("2021-10-31")
	test.Equal([]repository.AliasInterval{
		{ISIN: metaISIN, Symbol: "FB", FromDate: test.date("2021-09-01"), ToDate: &rename},
		{ISIN: metaISIN, Symbol: "META", FromDate: rename, ToDate: nil},
	}, test.metaAliases())
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

	// AAPL is OK, then the stream breaks. We can't tell which bars were
	// stored, so the whole backtest goes. A STATUS_ERROR is a different
	// path (no price data) and fails before a later stream error.
	clientStreamMock := new(financeServicePB.MockMarketdata_DownloadHistoricalDataClient[interface{}])
	clientStreamMock.On("Recv", mock.Anything).Return(&financeServicePB.DownloadHistoricalDataResponse{
		Symbol: "AAPL",
		Status: financeServicePB.DownloadHistoricalDataResponse_STATUS_OK,
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

func (test *BacktestCommandTest) TestDownloadFinancialDataRemovesBacktestWhenSymbolHasNoPriceData() {
	backtests := repository.Backtest{Conn: test.db}
	err := backtests.UpdatePeriod(context.TODO(), "test-backtest", &backtest.Backtest_Period{
		StartDate: &pb.Date{Year: 2022, Month: 1, Day: 1},
		EndDate:   &pb.Date{Year: 2022, Month: 12, Day: 31},
		Assets: []*backtest.Backtest_Period_Asset{
			{Symbol: "FB", Enabled: true},
		},
	})
	test.Require().NoError(err)

	clientStreamMock := new(financeServicePB.MockMarketdata_DownloadHistoricalDataClient[interface{}])
	clientStreamMock.On("Recv", mock.Anything).Return(&financeServicePB.DownloadHistoricalDataResponse{
		Symbol: "FB",
		Status: financeServicePB.DownloadHistoricalDataResponse_STATUS_ERROR,
		Error:  "no price data for FB",
	}, nil).Once()
	test.mdclient.On("DownloadHistoricalData", mock.Anything, mock.Anything).Return(clientStreamMock, nil)

	// Periods on the fixture span 2022-01-01 .. 2023-12-31.
	err = command.DownloadFinancialData(context.TODO(), test.downloadMessage("test-backtest"))
	test.Require().EqualError(err,
		"Couldn't load data for backtest 'test-backtest': no price data for FB between 2022-01-01 and 2023-12-31. "+
			"Nothing was saved, so run `fbull backtest create` again once it's fixed.")

	_, err = backtests.Get(context.TODO(), "test-backtest")
	test.Require().ErrorIs(err, pgx.ErrNoRows)
}

func (test *BacktestCommandTest) ingestMessage(name string, ziplineEngine engine.Engine) *stream.MockMessage {
	store := new(storage.MockStorage)
	store.On("CreateObject", mock.Anything, storage.IngestionsBucket, name, mock.Anything).
		Return(&storage.Object{Bucket: storage.IngestionsBucket, Name: name}, nil)
	msg := new(stream.MockMessage)
	msg.On("MustGet", stream.DBDep).Return(test.db)
	msg.On("MustGet", stream.StorageDep).Return(store)
	msg.On("Call", mock.Anything, dependency.GetEngineKey).Return(ziplineEngine, nil)
	msg.On("ParsePayload", &bs.IngestCommand{}).Return(nil).Run(func(args mock.Arguments) {
		payload := args.Get(0).(*bs.IngestCommand)
		payload.BacktestName = name
	})
	return msg
}

// An ingest error reported by the data service removes the backtest, its
// periods and its assets, so the same create works again without a delete.
// The step message is the user-facing copy and carries no password.
func (test *BacktestCommandTest) TestIngestErrorRemovesBacktest() {
	backtests := repository.Backtest{Conn: test.db}
	err := backtests.UpdatePeriod(context.TODO(), "test-backtest", &backtest.Backtest_Period{
		StartDate: &pb.Date{Year: 2022, Month: 1, Day: 1},
		EndDate:   &pb.Date{Year: 2022, Month: 12, Day: 31},
		Assets:    []*backtest.Backtest_Period_Asset{{Symbol: "AAPL", Enabled: true}},
	})
	test.Require().NoError(err)

	ziplineEngine := new(engine.MockEngine)
	ziplineEngine.On("Ingest", mock.Anything, mock.Anything, mock.Anything).Return(fmt.Errorf("error ingesting: %w",
		status.Error(codes.FailedPrecondition, "the data service is missing its database driver (psycopg)"+
			"\nDetails: ModuleNotFoundError: No module named 'psycopg' (postgresql://foreverbull:s3cr3t@postgres/x)")))

	err = command.Ingest(context.TODO(), test.ingestMessage("test-backtest", ziplineEngine))
	test.Require().EqualError(err, "Couldn't load data for backtest 'test-backtest': the data service is missing "+
		"its database driver (psycopg). This is a bug in the Foreverbull install, not in your strategy or settings. "+
		"Nothing was saved, so run `fbull backtest create` again once it's fixed."+
		"\nDetails: ModuleNotFoundError: No module named 'psycopg' (postgresql://foreverbull:***@postgres/x)")
	test.NotContains(err.Error(), "s3cr3t")

	_, err = backtests.Get(context.TODO(), "test-backtest")
	test.Require().ErrorIs(err, pgx.ErrNoRows)
	var periods, assets int
	test.Require().NoError(test.db.QueryRow(context.TODO(),
		`SELECT count(*) FROM backtest_period WHERE backtest_name=$1`, "test-backtest").Scan(&periods))
	test.Require().NoError(test.db.QueryRow(context.TODO(),
		`SELECT count(*) FROM backtest_period_asset WHERE backtest_name=$1`, "test-backtest").Scan(&assets))
	test.Zero(periods)
	test.Zero(assets)

	test.createAgain()
	b, err := backtests.Get(context.TODO(), "test-backtest")
	test.Require().NoError(err)
	test.Equal("test-backtest", b.Name)
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
