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

// A bar stored without a provider is written as supplier.DefaultProvider, never the column's 'yahoo' default.
func (test *OHLCTests) TestStoreEmptyProviderWritesDefaultProvider() {
	ctx := context.Background()
	day := time.Date(2021, 6, 3, 0, 0, 0, 0, time.UTC)
	test.Require().NoError(test.ohlcStorage.Store(ctx, "", "ABC", day, 1, 1, 1, 9, 1))

	var provider string
	err := test.conn.QueryRow(ctx, `SELECT provider FROM ohlc WHERE symbol = $1 AND time = $2`, "ABC", day).Scan(&provider)
	test.Require().NoError(err)
	test.Equal("stooq", provider)
	test.Equal(supplier.DefaultProvider, provider)
}
