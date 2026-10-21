package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/internal/test_helper"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/repository"
	"github.com/stretchr/testify/suite"
)

type InstrumentTest struct {
	suite.Suite

	conn        *pgxpool.Pool
	instruments repository.Instrument
}

func (test *InstrumentTest) SetupSuite() {
	test_helper.SetupEnvironment(test.T(), &test_helper.Containers{
		Postgres: true,
	})
}

func (test *InstrumentTest) SetupTest() {
	var err error
	test.conn, err = pgxpool.New(context.Background(), environment.GetPostgresURL())
	test.Require().NoError(err)
	err = repository.Recreate(context.Background(), test.conn)
	test.Require().NoError(err)
	test.instruments = repository.Instrument{Conn: test.conn}
}

func (test *InstrumentTest) TearDownTest() {
	test.conn.Close()
}

func TestInstrument(t *testing.T) {
	suite.Run(t, new(InstrumentTest))
}

func (test *InstrumentTest) date(s string) time.Time {
	d, err := time.Parse("2006-01-02", s)
	test.Require().NoError(err)
	return d
}

func (test *InstrumentTest) toPtr(s string) *time.Time {
	d := test.date(s)
	return &d
}

func (test *InstrumentTest) TestSymbolAtFollowsTickerChange() {
	ctx := context.Background()
	test.Require().NoError(test.instruments.Upsert(ctx, metaISIN, "Facebook Inc"))
	test.Require().NoError(test.instruments.StoreAlias(ctx, repository.AliasInterval{
		ISIN:     metaISIN,
		Symbol:   "FB",
		FromDate: test.date("2021-06-30"),
		ToDate:   test.toPtr("2021-10-31"),
	}))
	test.Require().NoError(test.instruments.StoreAlias(ctx, repository.AliasInterval{
		ISIN:     metaISIN,
		Symbol:   "META",
		FromDate: test.date("2021-10-31"),
		ToDate:   nil,
	}))

	symbol, err := test.instruments.SymbolAt(ctx, metaISIN, test.date("2021-06-30"))
	test.Require().NoError(err)
	test.Equal("FB", symbol)

	symbol, err = test.instruments.SymbolAt(ctx, metaISIN, test.date("2021-10-30"))
	test.Require().NoError(err)
	test.Equal("FB", symbol)

	symbol, err = test.instruments.SymbolAt(ctx, metaISIN, test.date("2021-10-31"))
	test.Require().NoError(err)
	test.Equal("META", symbol)

	_, err = test.instruments.SymbolAt(ctx, metaISIN, test.date("2021-06-29"))
	test.Require().ErrorIs(err, pgx.ErrNoRows)

	aliases, err := test.instruments.ListAliases(ctx, metaISIN)
	test.Require().NoError(err)
	test.Equal([]repository.AliasInterval{
		{ISIN: metaISIN, Symbol: "FB", FromDate: test.date("2021-06-30"), ToDate: test.toPtr("2021-10-31")},
		{ISIN: metaISIN, Symbol: "META", FromDate: test.date("2021-10-31"), ToDate: nil},
	}, aliases)
}

// asset_symbol is shared by every ETF backtest. Two backtests over different
// windows store the same ticker from different dates; that is one alias, not
// an overlap. A different ticker over the same dates is still refused.
func (test *InstrumentTest) TestStoreAliasMergesSameTickerAcrossBacktests() {
	ctx := context.Background()
	test.Require().NoError(test.instruments.Upsert(ctx, metaISIN, "Meta Platforms Inc"))
	store := func(symbol, from string, to *time.Time) error {
		return test.instruments.StoreAlias(ctx, repository.AliasInterval{
			ISIN: metaISIN, Symbol: symbol, FromDate: test.date(from), ToDate: to,
		})
	}

	test.Require().NoError(store("FB", "2020-01-31", nil))
	test.Require().NoError(store("FB", "2020-01-15", nil))
	test.Require().NoError(store("FB", "2020-06-30", nil))
	aliases, err := test.instruments.ListAliases(ctx, metaISIN)
	test.Require().NoError(err)
	test.Equal([]repository.AliasInterval{
		{ISIN: metaISIN, Symbol: "FB", FromDate: test.date("2020-01-15"), ToDate: nil},
	}, aliases)

	// A later window that sees the rename closes nothing it did not cover.
	_, err = test.conn.Exec(ctx, `DELETE FROM asset_symbol`)
	test.Require().NoError(err)
	test.Require().NoError(store("FB", "2021-06-30", test.toPtr("2021-10-31")))
	test.Require().NoError(store("FB", "2021-01-01", test.toPtr("2021-10-31")))
	test.Require().NoError(store("META", "2021-10-31", nil))
	aliases, err = test.instruments.ListAliases(ctx, metaISIN)
	test.Require().NoError(err)
	test.Equal([]repository.AliasInterval{
		{ISIN: metaISIN, Symbol: "FB", FromDate: test.date("2021-01-01"), ToDate: test.toPtr("2021-10-31")},
		{ISIN: metaISIN, Symbol: "META", FromDate: test.date("2021-10-31"), ToDate: nil},
	}, aliases)

	err = store("META", "2021-08-01", nil)
	test.Require().ErrorContains(err, "asset_symbol_no_overlap")
}

func (test *InstrumentTest) TestAssetSymbolRejectsOverlapAndAcceptsAdjacent() {
	ctx := context.Background()
	test.Require().NoError(repository.MigrateAssetSymbol(ctx, test.conn))
	test.Require().NoError(repository.MigrateAssetSymbol(ctx, test.conn))

	test.Require().NoError(test.instruments.Upsert(ctx, metaISIN, "Meta Platforms Inc"))
	_, err := test.conn.Exec(ctx, `
		INSERT INTO asset_symbol (isin, symbol, from_date, to_date)
		VALUES ($1, 'FB', '2021-06-30', '2021-10-31')`, metaISIN)
	test.Require().NoError(err)
	_, err = test.conn.Exec(ctx, `
		INSERT INTO asset_symbol (isin, symbol, from_date, to_date)
		VALUES ($1, 'META', '2021-08-01', NULL)`, metaISIN)
	test.Require().ErrorContains(err, "asset_symbol_no_overlap")
	_, err = test.conn.Exec(ctx, `
		INSERT INTO asset_symbol (isin, symbol, from_date, to_date)
		VALUES ($1, 'META', '2021-10-31', NULL)`, metaISIN)
	test.Require().NoError(err)

	_, err = test.conn.Exec(ctx, `
		INSERT INTO asset_symbol (isin, symbol, from_date)
		VALUES ('US0000000000', 'ZZZ', '2020-01-01')`)
	test.Require().ErrorContains(err, "asset_symbol_isin_fkey")
}

func (test *InstrumentTest) TestMigrateAssetSymbolLeavesOverlappingRows() {
	ctx := context.Background()
	_, err := test.conn.Exec(ctx, `DROP TABLE asset_symbol`)
	test.Require().NoError(err)
	_, err = test.conn.Exec(ctx, `
		CREATE TABLE asset_symbol (
			isin text NOT NULL,
			symbol text NOT NULL,
			from_date date NOT NULL,
			to_date date,
			PRIMARY KEY (isin, from_date)
		)`)
	test.Require().NoError(err)
	test.Require().NoError(test.instruments.Upsert(ctx, metaISIN, "Meta Platforms Inc"))
	_, err = test.conn.Exec(ctx, `
		INSERT INTO asset_symbol (isin, symbol, from_date, to_date) VALUES
		($1, 'FB', '2021-01-01', '2021-06-01'),
		($1, 'META', '2021-03-01', NULL)`, metaISIN)
	test.Require().NoError(err)

	err = repository.MigrateAssetSymbol(ctx, test.conn)
	test.Require().ErrorContains(err, "asset_symbol")

	var n int64
	err = test.conn.QueryRow(ctx, `SELECT COUNT(*) FROM asset_symbol WHERE isin=$1`, metaISIN).Scan(&n)
	test.Require().NoError(err)
	test.Equal(int64(2), n)
}

func (test *InstrumentTest) TestSymbolAtRejectsMissingISIN() {
	_, err := test.instruments.SymbolAt(context.Background(), "  ", test.date("2021-06-30"))
	test.Require().ErrorContains(err, "missing ISIN")
}
