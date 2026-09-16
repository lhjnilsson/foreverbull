package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/internal/test_helper"
	"github.com/lhjnilsson/foreverbull/pkg/finance/internal/repository"
	"github.com/stretchr/testify/suite"
)

const metaISIN = "US30303M1027"

type InstrumentTests struct {
	suite.Suite

	conn        *pgxpool.Pool
	instruments repository.Instruments
	assets      repository.Asset
	ohlc        repository.OHLC
}

func (test *InstrumentTests) SetupSuite() {
	test_helper.SetupEnvironment(test.T(), &test_helper.Containers{
		Postgres: true,
	})
}

func (test *InstrumentTests) SetupTest() {
	var err error
	test.conn, err = pgxpool.New(context.Background(), environment.GetPostgresURL())
	test.Require().NoError(err)
	err = repository.Recreate(context.Background(), test.conn)
	test.Require().NoError(err)
	test.instruments = repository.Instruments{Conn: test.conn}
	test.assets = repository.Asset{Conn: test.conn}
	test.ohlc = repository.OHLC{Conn: test.conn}
}

func (test *InstrumentTests) TearDownTest() {
	test.conn.Close()
}

func TestInstrument(t *testing.T) {
	suite.Run(t, new(InstrumentTests))
}

func (test *InstrumentTests) date(year int, month time.Month, day int) time.Time {
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}

func (test *InstrumentTests) TestStoreMissingISIN() {
	inst, err := test.instruments.Store(context.TODO(), "", "Meta Platforms Inc")
	test.Require().ErrorIs(err, repository.ErrMissingISIN)
	test.Nil(inst)

	inst, err = test.instruments.Store(context.TODO(), "   ", "Meta Platforms Inc")
	test.Require().ErrorIs(err, repository.ErrMissingISIN)
	test.Nil(inst)
}

func (test *InstrumentTests) TestStoreDoesNotMergeByName() {
	fb, err := test.instruments.Store(context.TODO(), metaISIN, "Meta Platforms Inc")
	test.Require().NoError(err)
	other, err := test.instruments.Store(context.TODO(), "US1234567890", "Meta Platforms Inc")
	test.Require().NoError(err)
	test.NotEqual(fb.ID, other.ID)
	test.NotEqual(fb.ISIN, other.ISIN)
	test.Equal(fb.Name, other.Name)

	same, err := test.instruments.Store(context.TODO(), metaISIN, "Facebook Inc")
	test.Require().NoError(err)
	test.Equal(fb.ID, same.ID)
	test.Equal(metaISIN, same.ISIN)
	test.Equal("Facebook Inc", same.Name)

	got, err := test.instruments.Get(context.TODO(), metaISIN)
	test.Require().NoError(err)
	test.Equal(fb.ID, got.ID)
	gotOther, err := test.instruments.Get(context.TODO(), "US1234567890")
	test.Require().NoError(err)
	test.Equal(other.ID, gotOther.ID)
}

func (test *InstrumentTests) TestTickerChangeIsNewAliasNotNewInstrument() {
	ctx := context.TODO()
	fbDate := test.date(2021, time.June, 30)
	metaDate := test.date(2021, time.October, 31)

	inst, err := test.instruments.Store(ctx, metaISIN, "Facebook Inc")
	test.Require().NoError(err)
	test.Require().NoError(test.instruments.RecordSymbol(ctx, metaISIN, "FB", fbDate))
	renamed, err := test.instruments.Store(ctx, metaISIN, "Meta Platforms Inc")
	test.Require().NoError(err)
	test.Equal(inst.ID, renamed.ID)
	test.Require().NoError(test.instruments.RecordSymbol(ctx, metaISIN, "META", metaDate))

	var n int64
	err = test.conn.QueryRow(ctx, `SELECT COUNT(*) FROM instrument WHERE isin=$1`, metaISIN).Scan(&n)
	test.Require().NoError(err)
	test.Equal(int64(1), n)

	err = test.conn.QueryRow(ctx, `SELECT COUNT(*) FROM asset_symbol WHERE isin=$1`, metaISIN).Scan(&n)
	test.Require().NoError(err)
	test.Equal(int64(2), n)

	resolvedFB, err := test.instruments.Resolve(ctx, "FB", fbDate)
	test.Require().NoError(err)
	test.Equal(inst.ID, resolvedFB.ID)
	test.Equal(metaISIN, resolvedFB.ISIN)

	resolvedMeta, err := test.instruments.Resolve(ctx, "META", metaDate)
	test.Require().NoError(err)
	test.Equal(inst.ID, resolvedMeta.ID)

	_, err = test.instruments.Resolve(ctx, "FB", metaDate)
	test.Require().ErrorIs(err, pgx.ErrNoRows)

	_, err = test.instruments.Resolve(ctx, "META", fbDate)
	test.Require().ErrorIs(err, pgx.ErrNoRows)

	symbol, err := test.instruments.Symbols(ctx, inst, fbDate)
	test.Require().NoError(err)
	test.Equal("FB", symbol)

	symbol, err = test.instruments.Symbols(ctx, renamed, metaDate)
	test.Require().NoError(err)
	test.Equal("META", symbol)

	symbol, err = test.instruments.Symbols(ctx, inst, metaDate.AddDate(0, 0, -1))
	test.Require().NoError(err)
	test.Equal("FB", symbol)
}

func (test *InstrumentTests) TestRecordSymbolMissingISIN() {
	err := test.instruments.RecordSymbol(context.TODO(), "", "FB", test.date(2021, time.June, 30))
	test.Require().ErrorIs(err, repository.ErrMissingISIN)
}

func (test *InstrumentTests) TestRecordSymbolSameTickerIsNoop() {
	ctx := context.TODO()
	d0 := test.date(2021, time.January, 31)
	d1 := test.date(2021, time.February, 28)

	_, err := test.instruments.Store(ctx, metaISIN, "Facebook Inc")
	test.Require().NoError(err)
	test.Require().NoError(test.instruments.RecordSymbol(ctx, metaISIN, "FB", d0))
	test.Require().NoError(test.instruments.RecordSymbol(ctx, metaISIN, "FB", d1))

	var n int64
	err = test.conn.QueryRow(ctx, `SELECT COUNT(*) FROM asset_symbol WHERE isin=$1`, metaISIN).Scan(&n)
	test.Require().NoError(err)
	test.Equal(int64(1), n)

	symbol, err := test.instruments.Symbols(ctx, &repository.Instrument{ISIN: metaISIN}, d1)
	test.Require().NoError(err)
	test.Equal("FB", symbol)
}

func (test *InstrumentTests) TestIngestAssetAndOHLCStillKeyedBySymbol() {
	ctx := context.TODO()
	_, err := test.instruments.Store(ctx, metaISIN, "Meta Platforms Inc")
	test.Require().NoError(err)
	test.Require().NoError(test.instruments.RecordSymbol(ctx, metaISIN, "FB", test.date(2021, time.June, 30)))
	test.Require().NoError(test.instruments.RecordSymbol(ctx, metaISIN, "META", test.date(2021, time.October, 31)))

	test.Require().NoError(test.assets.Store(ctx, "FB", "Facebook Inc"))
	test.Require().NoError(test.assets.Store(ctx, "META", "Meta Platforms Inc"))
	test.Require().NoError(test.ohlc.Store(ctx, "FB", test.date(2021, time.June, 30), 1, 1, 1, 1, 100))
	test.Require().NoError(test.ohlc.Store(ctx, "META", test.date(2021, time.October, 31), 2, 2, 2, 2, 200))

	fb, err := test.assets.Get(ctx, "FB")
	test.Require().NoError(err)
	test.Equal("FB", fb.Symbol)
	meta, err := test.assets.Get(ctx, "META")
	test.Require().NoError(err)
	test.Equal("META", meta.Symbol)

	var n int64
	err = test.conn.QueryRow(ctx, `SELECT COUNT(*) FROM ohlc WHERE symbol=$1`, "FB").Scan(&n)
	test.Require().NoError(err)
	test.Equal(int64(1), n)
	err = test.conn.QueryRow(ctx, `SELECT COUNT(*) FROM ohlc WHERE symbol=$1`, "META").Scan(&n)
	test.Require().NoError(err)
	test.Equal(int64(1), n)
}
