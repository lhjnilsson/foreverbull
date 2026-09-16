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

func (test *InstrumentTest) TestSymbolAtRejectsMissingISIN() {
	_, err := test.instruments.SymbolAt(context.Background(), "  ", test.date("2021-06-30"))
	test.Require().ErrorContains(err, "missing ISIN")
}
