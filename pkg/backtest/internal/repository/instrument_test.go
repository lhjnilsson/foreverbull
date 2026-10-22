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

func (test *InstrumentTest) storeFunc(ctx context.Context) func(symbol, from string, to *time.Time) error {
	return func(symbol, from string, to *time.Time) error {
		return test.instruments.StoreAlias(ctx, repository.AliasInterval{
			ISIN: metaISIN, Symbol: symbol, FromDate: test.date(from), ToDate: to,
		})
	}
}

func (test *InstrumentTest) requireAliases(ctx context.Context, want []repository.AliasInterval) {
	aliases, err := test.instruments.ListAliases(ctx, metaISIN)
	test.Require().NoError(err)
	test.Equal(want, aliases)
}

// A backtest that ends before a rename leaves FB open-ended. A later backtest
// that sees the rename closes it where META starts, and its own FB rows join it.
func (test *InstrumentTest) TestStoreAliasClosesOpenAliasAtLaterRename() {
	ctx := context.Background()
	test.Require().NoError(test.instruments.Upsert(ctx, metaISIN, "Meta Platforms Inc"))
	store := test.storeFunc(ctx)

	test.Require().NoError(store("FB", "2021-01-01", nil))
	test.Require().NoError(store("FB", "2021-06-30", test.toPtr("2021-10-31")))
	test.Require().NoError(store("META", "2021-10-31", nil))
	test.requireAliases(ctx, []repository.AliasInterval{
		{ISIN: metaISIN, Symbol: "FB", FromDate: test.date("2021-01-01"), ToDate: test.toPtr("2021-10-31")},
		{ISIN: metaISIN, Symbol: "META", FromDate: test.date("2021-10-31"), ToDate: nil},
	})

	symbol, err := test.instruments.SymbolAt(ctx, metaISIN, test.date("2021-10-30"))
	test.Require().NoError(err)
	test.Equal("FB", symbol)
	symbol, err = test.instruments.SymbolAt(ctx, metaISIN, test.date("2021-10-31"))
	test.Require().NoError(err)
	test.Equal("META", symbol)
}

// A later backtest that starts after the rename only sees META. The open FB
// row ends where META starts; nothing is known about the days in between.
func (test *InstrumentTest) TestStoreAliasClosesOpenAliasAtLaterTicker() {
	ctx := context.Background()
	test.Require().NoError(test.instruments.Upsert(ctx, metaISIN, "Meta Platforms Inc"))
	store := test.storeFunc(ctx)

	test.Require().NoError(store("FB", "2021-01-01", nil))
	test.Require().NoError(store("META", "2022-01-31", nil))
	test.requireAliases(ctx, []repository.AliasInterval{
		{ISIN: metaISIN, Symbol: "FB", FromDate: test.date("2021-01-01"), ToDate: test.toPtr("2022-01-31")},
		{ISIN: metaISIN, Symbol: "META", FromDate: test.date("2022-01-31"), ToDate: nil},
	})
}

// Reverse order: the newer backtest already stored the rename. An older one
// that sees FB open-ended still creates; its FB ends where META starts.
func (test *InstrumentTest) TestStoreAliasCapsOpenAliasAtStoredRename() {
	ctx := context.Background()
	test.Require().NoError(test.instruments.Upsert(ctx, metaISIN, "Meta Platforms Inc"))
	store := test.storeFunc(ctx)

	test.Require().NoError(store("FB", "2021-06-30", test.toPtr("2021-10-31")))
	test.Require().NoError(store("META", "2021-10-31", nil))

	test.Require().NoError(store("FB", "2021-01-01", nil))
	want := []repository.AliasInterval{
		{ISIN: metaISIN, Symbol: "FB", FromDate: test.date("2021-01-01"), ToDate: test.toPtr("2021-10-31")},
		{ISIN: metaISIN, Symbol: "META", FromDate: test.date("2021-10-31"), ToDate: nil},
	}
	test.requireAliases(ctx, want)

	// An open FB inside the stored FB row changes nothing.
	test.Require().NoError(store("FB", "2021-08-31", nil))
	test.requireAliases(ctx, want)

	// The cap is the next row of another ticker, not the last one.
	test.Require().NoError(store("META", "2021-10-31", test.toPtr("2022-06-30")))
	test.Require().NoError(store("FB", "2022-06-30", nil))
	test.requireAliases(ctx, []repository.AliasInterval{
		{ISIN: metaISIN, Symbol: "FB", FromDate: test.date("2021-01-01"), ToDate: test.toPtr("2021-10-31")},
		{ISIN: metaISIN, Symbol: "META", FromDate: test.date("2021-10-31"), ToDate: test.toPtr("2022-06-30")},
		{ISIN: metaISIN, Symbol: "FB", FromDate: test.date("2022-06-30"), ToDate: nil},
	})
}

// Two tickers for one ISIN on the same day from published lists is a real
// conflict. It is refused and the stored row is kept, never replaced.
func (test *InstrumentTest) TestStoreAliasRefusesDifferentTickerOnSameDay() {
	ctx := context.Background()
	test.Require().NoError(test.instruments.Upsert(ctx, metaISIN, "Meta Platforms Inc"))
	store := test.storeFunc(ctx)

	test.Require().NoError(store("FB", "2021-10-31", nil))
	err := store("META", "2021-10-31", nil)
	test.Require().ErrorIs(err, repository.ErrAliasConflict)
	test.Require().ErrorContains(err, "asset_symbol_no_overlap")
	err = store("META", "2021-10-31", test.toPtr("2021-11-30"))
	test.Require().ErrorIs(err, repository.ErrAliasConflict)
	test.requireAliases(ctx, []repository.AliasInterval{
		{ISIN: metaISIN, Symbol: "FB", FromDate: test.date("2021-10-31"), ToDate: nil},
	})

	// A published FB interval is not shortened by a META that starts inside it.
	_, err = test.conn.Exec(ctx, `DELETE FROM asset_symbol`)
	test.Require().NoError(err)
	test.Require().NoError(store("FB", "2021-06-30", test.toPtr("2021-10-31")))
	err = store("META", "2021-09-30", nil)
	test.Require().ErrorIs(err, repository.ErrAliasConflict)
	err = store("META", "2021-09-30", test.toPtr("2021-12-31"))
	test.Require().ErrorIs(err, repository.ErrAliasConflict)

	// An open FB is not cut before its own start.
	_, err = test.conn.Exec(ctx, `DELETE FROM asset_symbol`)
	test.Require().NoError(err)
	test.Require().NoError(store("FB", "2021-06-30", nil))
	err = store("META", "2021-03-31", test.toPtr("2021-12-31"))
	test.Require().ErrorIs(err, repository.ErrAliasConflict)
	test.requireAliases(ctx, []repository.AliasInterval{
		{ISIN: metaISIN, Symbol: "FB", FromDate: test.date("2021-06-30"), ToDate: nil},
	})
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
