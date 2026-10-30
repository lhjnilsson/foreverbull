package repository_test

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/internal/test_helper"
	"github.com/lhjnilsson/foreverbull/pkg/finance/internal/repository"
	"github.com/lhjnilsson/foreverbull/pkg/finance/supplier"
	internal_pb "github.com/lhjnilsson/foreverbull/pkg/pb"
	pb "github.com/lhjnilsson/foreverbull/pkg/pb/finance"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type OHLCTests struct {
	suite.Suite
	conn        *pgxpool.Pool
	asset       pb.Asset
	ohlcStorage repository.OHLC
}

func (test *OHLCTests) SetupSuite() {
	test_helper.SetupEnvironment(test.T(), &test_helper.Containers{
		Postgres: true,
	})
}

func (test *OHLCTests) SetupTest() {
	var err error
	test.conn, err = pgxpool.New(context.Background(), environment.GetPostgresURL())
	test.Require().NoError(err)
	err = repository.Recreate(context.Background(), test.conn)
	test.Require().NoError(err)
	assetStorage := repository.Asset{Conn: test.conn}
	test.asset = pb.Asset{Symbol: "ABC", Name: "Company ABC"}
	err = assetStorage.Store(context.TODO(), test.asset.Symbol, test.asset.Name)
	test.Require().NoError(err)

	_, err = test.conn.Exec(context.TODO(), "DROP TABLE IF EXISTS ohlc;")
	test.Require().NoError(err)
	_, err = test.conn.Exec(context.TODO(), repository.OHLCTable)
	test.Require().NoError(err)
	test.ohlcStorage = repository.OHLC{Conn: test.conn}
}

func (test *OHLCTests) TearDownTest() {
	test.conn.Close()
}

func TestOHLC(t *testing.T) {
	suite.Run(t, new(OHLCTests))
}

func TestStoreRejectsNaNOrNegativeClose(t *testing.T) {
	db := repository.OHLC{}
	day := time.Date(2020, 1, 2, 0, 0, 0, 0, time.UTC)
	cases := []struct {
		name    string
		closePx float64
	}{
		{name: "nan", closePx: math.NaN()},
		{name: "zero", closePx: 0},
		{name: "negative", closePx: -1},
		{name: "negative infinity", closePx: math.Inf(-1)},
		{name: "positive infinity", closePx: math.Inf(1)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := db.Store(context.Background(), "yahoo", "ABC", day, 1.2, 1.3, 1.1, tc.closePx, 1000)
			require.ErrorIs(t, err, supplier.ErrRejected)
		})
	}
}

// A missing provider bar used to arrive as 0 in every price column. No
// price column may be stored as 0, negative, or non-finite.
func TestStoreRejectsNonPositiveOpenHighLow(t *testing.T) {
	db := repository.OHLC{}
	day := time.Date(2020, 1, 2, 0, 0, 0, 0, time.UTC)
	cases := []struct {
		name            string
		open, high, low float64
		wantErrorPrefix string
	}{
		{name: "zero open", open: 0, high: 1.3, low: 1.1, wantErrorPrefix: "open"},
		{name: "zero high", open: 1.2, high: 0, low: 1.1, wantErrorPrefix: "high"},
		{name: "zero low", open: 1.2, high: 1.3, low: 0, wantErrorPrefix: "low"},
		{name: "nan open", open: math.NaN(), high: 1.3, low: 1.1, wantErrorPrefix: "open"},
		{name: "negative low", open: 1.2, high: 1.3, low: -1, wantErrorPrefix: "low"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := db.Store(context.Background(), "yahoo", "ABC", day, tc.open, tc.high, tc.low, 1.2, 1000)
			require.ErrorIs(t, err, supplier.ErrRejected)
			require.ErrorContains(t, err, tc.wantErrorPrefix+":")
		})
	}
}

func (test *OHLCTests) SampleOHLC() (string, *internal_pb.Date, *internal_pb.Date) {
	count := 5
	ohlcStart := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	ohlcTime := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)

	for i := 0; i <= count; i++ {
		err := test.ohlcStorage.Store(context.TODO(), "", test.asset.Symbol, ohlcTime, 1.2, 1.3, 1.1, 1.2, 1000)
		test.Require().NoError(err)

		if i != count {
			ohlcTime = ohlcTime.Add(time.Hour * 24)
		}
	}

	return test.asset.Symbol, internal_pb.GoTimeToDate(ohlcStart), internal_pb.GoTimeToDate(ohlcTime)
}

func (test *OHLCTests) TestStore() {
	err := test.ohlcStorage.Store(context.TODO(), "", test.asset.Symbol, time.Now(), 1.2, 1.3, 1.1, 1.2, 1000)
	test.Require().NoError(err)
}

func (test *OHLCTests) TestExists() {
	symbol, start, end := test.SampleOHLC()
	exists, err := test.ohlcStorage.Exists(context.TODO(), "", []string{symbol}, start, end)
	test.Require().NoError(err)
	test.True(exists)
}

func (test *OHLCTests) TestExistsNot() {
	symbol, start, end := test.SampleOHLC()
	end.Day++
	exists, err := test.ohlcStorage.Exists(context.TODO(), "", []string{symbol}, start, end)
	test.Require().NoError(err)
	test.False(exists)
}

func (test *OHLCTests) TestMinMaxNothingStored() {
	storedMin, storedMax, err := test.ohlcStorage.MinMax(context.Background())
	test.Require().NoError(err)
	test.Nil(storedMin)
	test.Nil(storedMax)
}

func (test *OHLCTests) TestYahooReadReturnsOnlyYahooClose() {
	ctx := context.Background()
	_, err := test.conn.Exec(ctx, `DROP TABLE ohlc`)
	test.Require().NoError(err)
	_, err = test.conn.Exec(ctx, `
		CREATE TABLE ohlc (
			symbol TEXT REFERENCES asset(symbol),
			open numeric,
			high numeric,
			low numeric,
			close numeric,
			volume bigint,
			time timestamp,
			UNIQUE (symbol, time)
		)`)
	test.Require().NoError(err)

	day := time.Date(2021, 6, 1, 15, 0, 0, 0, time.UTC)
	_, err = test.conn.Exec(ctx, `
		INSERT INTO ohlc (symbol, time, open, high, low, close, volume)
		VALUES ($1, $2, 1, 1, 1, 11, 1)`, "ABC", day)
	test.Require().NoError(err)

	test.Require().NoError(repository.MigrateOHLC(ctx, test.conn))
	test.Require().NoError(repository.MigrateOHLC(ctx, test.conn))

	var storedProvider string
	var storedClose float64
	err = test.conn.QueryRow(ctx, `SELECT provider, close::float8 FROM ohlc WHERE symbol = $1`, "ABC").Scan(&storedProvider, &storedClose)
	test.Require().NoError(err)
	test.Equal("yahoo", storedProvider)
	test.InDelta(11, storedClose, 0.0001)

	// Old rows were filled with yahoo. A new row must name its provider.
	_, err = test.conn.Exec(ctx, `
		INSERT INTO ohlc (symbol, time, open, high, low, close, volume)
		VALUES ($1, $2, 1, 1, 1, 12, 1)`, "ABC", day.Add(24*time.Hour))
	test.Require().ErrorContains(err, "provider")

	err = test.ohlcStorage.Store(ctx, "stooq", "ABC", day, 1, 2, 0.5, 22, 3)
	test.Require().NoError(err)

	got, err := test.ohlcStorage.Close(ctx, "yahoo", "ABC", day)
	test.Require().NoError(err)
	test.InDelta(11, got, 0.0001)

	other, err := test.ohlcStorage.Close(ctx, "stooq", "ABC", day)
	test.Require().NoError(err)
	test.InDelta(22, other, 0.0001)

	var n int
	err = test.conn.QueryRow(ctx, `SELECT count(*) FROM ohlc WHERE symbol = $1`, "ABC").Scan(&n)
	test.Require().NoError(err)
	test.Equal(2, n)
}

// A bar that omits provider must fail loudly rather than be stored as yahoo.
func (test *OHLCTests) TestInsertWithoutProviderFails() {
	ctx := context.Background()
	_, err := test.conn.Exec(ctx, `
		INSERT INTO ohlc (symbol, time, open, high, low, close, volume)
		VALUES ('ABC', '2020-01-02', 1, 1, 1, 1, 1)`)
	test.Require().ErrorContains(err, "provider")

	var count int
	err = test.conn.QueryRow(ctx, `SELECT count(*) FROM ohlc`).Scan(&count)
	test.Require().NoError(err)
	test.Equal(0, count)
}

func (test *OHLCTests) TestMigrateDropsSymbolTimeUC() {
	ctx := context.Background()
	_, err := test.conn.Exec(ctx, `DROP TABLE ohlc`)
	test.Require().NoError(err)
	_, err = test.conn.Exec(ctx, `
		CREATE TABLE ohlc (
			symbol TEXT REFERENCES asset(symbol),
			open numeric,
			high numeric,
			low numeric,
			close numeric,
			volume bigint,
			time timestamp
		)`)
	test.Require().NoError(err)
	_, err = test.conn.Exec(ctx, `ALTER TABLE ohlc ADD CONSTRAINT symbol_time_uc UNIQUE (symbol, time)`)
	test.Require().NoError(err)
	day := time.Date(2021, 6, 2, 0, 0, 0, 0, time.UTC)
	_, err = test.conn.Exec(ctx, `
		INSERT INTO ohlc (symbol, time, open, high, low, close, volume)
		VALUES ($1, $2, 1, 1, 1, 7, 1)`, "ABC", day)
	test.Require().NoError(err)

	test.Require().NoError(repository.MigrateOHLC(ctx, test.conn))

	var provider string
	err = test.conn.QueryRow(ctx, `SELECT provider FROM ohlc WHERE symbol = $1`, "ABC").Scan(&provider)
	test.Require().NoError(err)
	test.Equal("yahoo", provider)

	err = test.ohlcStorage.Store(ctx, "stooq", "ABC", day, 1, 1, 1, 8, 1)
	test.Require().NoError(err)
	got, err := test.ohlcStorage.Close(ctx, "yahoo", "ABC", day)
	test.Require().NoError(err)
	test.InDelta(7, got, 0.0001)
}

func (test *OHLCTests) TestBarsRejectInvalidRows() {
	ctx := context.Background()
	test.Require().NoError(repository.MigrateOHLCBars(ctx, test.conn))
	test.Require().NoError(repository.MigrateOHLCBars(ctx, test.conn))

	_, err := test.conn.Exec(ctx, `
		INSERT INTO ohlc (provider, symbol, time, open, high, low, close, volume)
		VALUES ('yahoo', 'ABC', '2020-01-02', 1, 1, 1, 1, 0)`)
	test.Require().NoError(err)
	_, err = test.conn.Exec(ctx, `
		INSERT INTO ohlc (provider, symbol, time, open, high, low, close, volume)
		VALUES ('stooq', 'ABC', '2020-01-03', 1, 2, 2, 2, 1)`)
	test.Require().NoError(err)
	_, err = test.conn.Exec(ctx, `
		INSERT INTO ohlc (provider, symbol, time, open, high, low, close, volume)
		VALUES ('alpaca', 'ABC', '2020-01-04', 1, 2, 1, 2, 1)`)
	test.Require().NoError(err)

	_, err = test.conn.Exec(ctx, `
		INSERT INTO ohlc (provider, symbol, time, open, high, low, close, volume)
		VALUES ('yahoo', 'ABC', '2020-01-05', 1, 1, 1, NULL, 1)`)
	test.Require().ErrorContains(err, "ohlc")
	_, err = test.conn.Exec(ctx, `
		INSERT INTO ohlc (provider, symbol, time, open, high, low, close, volume)
		VALUES ('yahoo', 'ABC', '2020-01-06', 1, 1, 2, 1, 1)`)
	test.Require().ErrorContains(err, "ohlc_bar_check")
	_, err = test.conn.Exec(ctx, `
		INSERT INTO ohlc (provider, symbol, time, open, high, low, close, volume)
		VALUES ('yahoo', 'ABC', '2020-01-07', 1, 1, 1, 1, -1)`)
	test.Require().ErrorContains(err, "ohlc_bar_check")
	_, err = test.conn.Exec(ctx, `
		INSERT INTO ohlc (provider, symbol, time, open, high, low, close, volume)
		VALUES ('nasdaq', 'ABC', '2020-01-08', 1, 1, 1, 1, 1)`)
	test.Require().ErrorContains(err, "ohlc_provider_check")
}

func (test *OHLCTests) TestMigrateOHLCBarsUpgradesOldTable() {
	ctx := context.Background()
	_, err := test.conn.Exec(ctx, `DROP TABLE ohlc`)
	test.Require().NoError(err)
	_, err = test.conn.Exec(ctx, `
		CREATE TABLE ohlc (
			symbol TEXT REFERENCES asset(symbol),
			open numeric,
			high numeric,
			low numeric,
			close numeric,
			volume bigint,
			time timestamp,
			provider text NOT NULL DEFAULT 'yahoo',
			UNIQUE (provider, symbol, time)
		)`)
	test.Require().NoError(err)
	_, err = test.conn.Exec(ctx, `
		INSERT INTO ohlc (symbol, time, open, high, low, close, volume)
		VALUES ('ABC', '2020-01-02', 1, 1, 1, 4, 1)`)
	test.Require().NoError(err)

	test.Require().NoError(repository.MigrateOHLCBars(ctx, test.conn))
	test.Require().NoError(repository.MigrateOHLCBars(ctx, test.conn))

	_, err = test.conn.Exec(ctx, `
		INSERT INTO ohlc (provider, symbol, time, open, high, low, close, volume)
		VALUES ('yahoo', 'ABC', '2020-01-03', 1, 1, 1, NULL, 1)`)
	test.Require().ErrorContains(err, "ohlc")
	_, err = test.conn.Exec(ctx, `
		INSERT INTO ohlc (provider, symbol, time, open, high, low, close, volume)
		VALUES ('nasdaq', 'ABC', '2020-01-04', 1, 1, 2, 1, 1)`)
	test.Require().ErrorContains(err, "ohlc")
}

func (test *OHLCTests) TestMigrateOHLCBarsLeavesViolatingRows() {
	ctx := context.Background()
	_, err := test.conn.Exec(ctx, `DROP TABLE ohlc`)
	test.Require().NoError(err)
	_, err = test.conn.Exec(ctx, `
		CREATE TABLE ohlc (
			symbol TEXT REFERENCES asset(symbol),
			open numeric,
			high numeric,
			low numeric,
			close numeric,
			volume bigint,
			time timestamp,
			provider text NOT NULL DEFAULT 'yahoo',
			UNIQUE (provider, symbol, time)
		)`)
	test.Require().NoError(err)
	_, err = test.conn.Exec(ctx, `
		INSERT INTO ohlc (symbol, time, open, high, low, close, volume, provider)
		VALUES ('ABC', '2020-01-02', 1, 1, 2, 1, -1, 'nasdaq')`)
	test.Require().NoError(err)

	err = repository.MigrateOHLCBars(ctx, test.conn)
	test.Require().ErrorContains(err, "ohlc")

	var n int64
	var low float64
	var volume int64
	var provider string
	err = test.conn.QueryRow(ctx, `
		SELECT COUNT(*), MIN(low)::float8, MIN(volume), MIN(provider) FROM ohlc`).Scan(&n, &low, &volume, &provider)
	test.Require().NoError(err)
	test.Equal(int64(1), n)
	test.InDelta(2, low, 0.0001)
	test.Equal(int64(-1), volume)
	test.Equal("nasdaq", provider)
}

func (test *OHLCTests) TestMinMax() {
	_, start, end := test.SampleOHLC()
	storedMin, storedMax, err := test.ohlcStorage.MinMax(context.Background())
	test.Require().NoError(err)
	test.Equal(start, storedMin)
	test.Equal(end, storedMax)
}

// Re-ingesting adjusted bars over stored ones leaves only the new scale.
func (test *OHLCTests) TestStoreReplacesBarsOnReingest() {
	ctx := context.Background()
	day := time.Date(2020, 8, 27, 0, 0, 0, 0, time.UTC)
	// Unadjusted, before a 4:1 split.
	for i, c := range []float64{500, 499.3} {
		test.Require().NoError(test.ohlcStorage.Store(ctx, "yahoo", "ABC", day.AddDate(0, 0, i), c, c, c, c, 100))
	}
	// Re-fetch after the split: same days on the new scale plus one new day.
	adjusted := []float64{125, 124.825, 129.04}
	for i, c := range adjusted {
		test.Require().NoError(test.ohlcStorage.Store(ctx, "yahoo", "ABC", day.AddDate(0, 0, i), c, c+1, c-1, c, 400))
	}

	rows, err := test.conn.Query(ctx,
		`SELECT open::float8, high::float8, low::float8, close::float8, volume FROM ohlc
		 WHERE provider = 'yahoo' AND symbol = 'ABC' ORDER BY time`)
	test.Require().NoError(err)
	defer rows.Close()
	var got int
	for rows.Next() {
		var o, h, l, c float64
		var v int
		test.Require().NoError(rows.Scan(&o, &h, &l, &c, &v))
		test.InDelta(adjusted[got], o, 0.0001)
		test.InDelta(adjusted[got]+1, h, 0.0001)
		test.InDelta(adjusted[got]-1, l, 0.0001)
		test.InDelta(adjusted[got], c, 0.0001)
		test.Equal(400, v)
		got++
	}
	test.Require().NoError(rows.Err())
	test.Equal(len(adjusted), got)
}

// A re-fetched bar with a bad price never replaces a good stored bar.
func (test *OHLCTests) TestStoreKeepsGoodBarOverBadRefetch() {
	ctx := context.Background()
	day := time.Date(2020, 8, 27, 0, 0, 0, 0, time.UTC)
	test.Require().NoError(test.ohlcStorage.Store(ctx, "yahoo", "ABC", day, 125, 126, 124, 125.5, 400))

	bad := map[string][4]float64{
		"zero open":     {0, 126, 124, 125},
		"negative high": {125, -1, 124, 125},
		"zero low":      {125, 126, 0, 125},
		"zero close":    {125, 126, 124, 0},
		"NaN open":      {math.NaN(), 126, 124, 125},
		"NaN low":       {125, 126, math.NaN(), 125},
		"Inf high":      {125, math.Inf(1), 124, 125},
	}
	for name, p := range bad {
		_ = test.ohlcStorage.Store(ctx, "yahoo", "ABC", day, p[0], p[1], p[2], p[3], 1)

		var o, h, l, c float64
		var v int
		test.Require().NoError(test.conn.QueryRow(ctx,
			`SELECT open::float8, high::float8, low::float8, close::float8, volume FROM ohlc
			 WHERE provider = 'yahoo' AND symbol = 'ABC' AND time = $1`, day).Scan(&o, &h, &l, &c, &v), name)
		test.InDelta(125, o, 0.0001, name)
		test.InDelta(126, h, 0.0001, name)
		test.InDelta(124, l, 0.0001, name)
		test.InDelta(125.5, c, 0.0001, name)
		test.Equal(400, v, name)
	}
}

// A bar stored without a provider is written as supplier.DefaultProvider.
func (test *OHLCTests) TestStoreEmptyProviderWritesDefaultProvider() {
	ctx := context.Background()
	day := time.Date(2021, 6, 3, 0, 0, 0, 0, time.UTC)
	test.Require().NoError(test.ohlcStorage.Store(ctx, "", "ABC", day, 1, 1, 1, 9, 1))

	var provider string
	err := test.conn.QueryRow(ctx, `SELECT provider FROM ohlc WHERE symbol = $1 AND time = $2`, "ABC", day).Scan(&provider)
	test.Require().NoError(err)
	test.Equal("yahoo", provider)
	test.Equal(supplier.DefaultProvider, provider)
}

func (test *OHLCTests) TestListRespectsAsOfLimitAndProvider() {
	ctx := context.Background()
	day := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 5; i++ {
		err := test.ohlcStorage.Store(ctx, "yahoo", "ABC", day.AddDate(0, 0, i), 1, 2, 0.5, float64(10+i), 100)
		test.Require().NoError(err)
		err = test.ohlcStorage.Store(ctx, "stooq", "ABC", day.AddDate(0, 0, i), 1, 2, 0.5, float64(20+i), 100)
		test.Require().NoError(err)
	}

	asOf := day.AddDate(0, 0, 3) // includes days 0..3
	bars, err := test.ohlcStorage.List(ctx, "yahoo", []string{"ABC"}, asOf, 2)
	test.Require().NoError(err)
	test.Require().Len(bars, 2)
	test.Equal("ABC", bars[0].Symbol)
	test.InDelta(12, bars[0].Close, 0.0001) // day+2
	test.InDelta(13, bars[1].Close, 0.0001) // day+3
	test.True(bars[0].Timestamp.AsTime().Before(bars[1].Timestamp.AsTime()) || bars[0].Timestamp.AsTime().Equal(bars[1].Timestamp.AsTime()))

	stooq, err := test.ohlcStorage.List(ctx, "stooq", []string{"ABC"}, asOf, 1)
	test.Require().NoError(err)
	test.Require().Len(stooq, 1)
	test.InDelta(23, stooq[0].Close, 0.0001)

	missing, err := test.ohlcStorage.List(ctx, "yahoo", []string{"NOPE"}, asOf, 10)
	test.Require().NoError(err)
	test.Empty(missing)
}

// A share class is stored as BRKB; a read for BRK.B or BRK-B finds those
// rows. VOD.L is not a listed class and is read exactly as typed.
func (test *OHLCTests) TestListReadsShareClassUnderStoredName() {
	ctx := context.Background()
	assets := repository.Asset{Conn: test.conn}
	test.Require().NoError(assets.Store(ctx, "BRKB", "Berkshire Hathaway B"))
	test.Require().NoError(assets.Store(ctx, "VOD.L", "Vodafone"))
	day := time.Date(2021, 3, 1, 21, 0, 0, 0, time.UTC)
	test.Require().NoError(test.ohlcStorage.Store(ctx, "yahoo", "BRKB", day, 1, 2, 0.5, 250, 10))
	test.Require().NoError(test.ohlcStorage.Store(ctx, "yahoo", "VOD.L", day, 1, 2, 0.5, 90, 10))

	for _, asked := range []string{"BRK.B", "BRK-B", "BRKB"} {
		bars, err := test.ohlcStorage.List(ctx, "yahoo", []string{asked}, day, 10)
		test.Require().NoError(err, asked)
		test.Require().Len(bars, 1, asked)
		test.Equal("BRKB", bars[0].GetSymbol(), asked)
		test.InDelta(250, bars[0].GetClose(), 0.0001, asked)

		resolved, err := test.ohlcStorage.ListResolved(ctx, "yahoo", []string{asked}, day, 10)
		test.Require().NoError(err, asked)
		test.Require().Len(resolved, 1, asked)
		test.Equal("BRKB", resolved[0].GetSymbol(), asked)
	}

	both, err := test.ohlcStorage.List(ctx, "yahoo", []string{"BRK.B", "BRKB"}, day, 10)
	test.Require().NoError(err)
	test.Len(both, 1)

	vod, err := test.ohlcStorage.List(ctx, "yahoo", []string{"VOD.L"}, day, 10)
	test.Require().NoError(err)
	test.Require().Len(vod, 1)
	test.Equal("VOD.L", vod[0].GetSymbol())
	none, err := test.ohlcStorage.List(ctx, "yahoo", []string{"VODL"}, day, 10)
	test.Require().NoError(err)
	test.Empty(none)
}

func (test *OHLCTests) TestListResolvedFollowsTickerAliases() {
	ctx := context.Background()
	instruments := repository.Instruments{Conn: test.conn}
	_, err := instruments.Store(ctx, "US30303M1027", "Meta Platforms")
	test.Require().NoError(err)
	assets := repository.Asset{Conn: test.conn}
	test.Require().NoError(assets.Store(ctx, "FB", "Facebook"))
	test.Require().NoError(assets.Store(ctx, "META", "Meta Platforms"))
	_, err = test.conn.Exec(ctx, `
		INSERT INTO asset_symbol (isin, symbol, from_date, to_date) VALUES
		($1, 'FB', '2021-07-01', '2021-10-31'),
		($1, 'META', '2021-10-31', NULL)`, "US30303M1027")
	test.Require().NoError(err)

	oct := time.Date(2021, 10, 1, 21, 0, 0, 0, time.UTC)
	nov := time.Date(2021, 11, 1, 21, 0, 0, 0, time.UTC)
	test.Require().NoError(test.ohlcStorage.Store(ctx, "yahoo", "FB", oct, 1, 1, 1, 10, 1))
	test.Require().NoError(test.ohlcStorage.Store(ctx, "yahoo", "META", nov, 1, 1, 1, 20, 1))
	test.Require().NoError(test.ohlcStorage.Store(ctx, "yahoo", "ABC", oct, 1, 1, 1, 7, 1))

	asOf := time.Date(2021, 11, 15, 21, 0, 0, 0, time.UTC)
	bars, err := test.ohlcStorage.ListResolved(ctx, "yahoo", []string{"META"}, asOf, 252)
	test.Require().NoError(err)
	test.Require().Len(bars, 2)
	test.Equal("META", bars[0].GetSymbol())
	test.InDelta(10, bars[0].GetClose(), 0.0001)
	test.Equal("META", bars[1].GetSymbol())
	test.InDelta(20, bars[1].GetClose(), 0.0001)
	test.NotContains(bars[0].String(), "postgres://")

	before := time.Date(2021, 10, 15, 21, 0, 0, 0, time.UTC)
	fb, err := test.ohlcStorage.ListResolved(ctx, "yahoo", []string{"FB"}, before, 252)
	test.Require().NoError(err)
	test.Require().Len(fb, 1)
	test.Equal("FB", fb[0].GetSymbol())
	test.InDelta(10, fb[0].GetClose(), 0.0001)

	own, err := test.ohlcStorage.ListResolved(ctx, "yahoo", []string{"ABC"}, asOf, 10)
	test.Require().NoError(err)
	test.Require().Len(own, 1)
	test.Equal("ABC", own[0].GetSymbol())
	test.InDelta(7, own[0].GetClose(), 0.0001)

	none, err := test.ohlcStorage.ListResolved(ctx, "yahoo", nil, asOf, 10)
	test.Require().NoError(err)
	test.Empty(none)
}

func (test *OHLCTests) TestListRejectsBadInput() {
	ctx := context.Background()
	asOf := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	_, err := test.ohlcStorage.List(ctx, "yahoo", nil, asOf, 1)
	test.Require().ErrorContains(err, "symbols")
	_, err = test.ohlcStorage.List(ctx, "yahoo", []string{"ABC"}, asOf, 3000)
	test.Require().ErrorContains(err, "limit")
}
