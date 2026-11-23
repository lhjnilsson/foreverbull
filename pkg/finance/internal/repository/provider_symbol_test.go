package repository_test

import (
	"bytes"
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/lhjnilsson/foreverbull/pkg/finance/internal/repository"
	financepb "github.com/lhjnilsson/foreverbull/pkg/pb/finance"
)

func (test *InstrumentTests) bar(symbol string, day time.Time, close float64) *financepb.OHLC {
	return &financepb.OHLC{
		Symbol: symbol, Timestamp: timestamppb.New(day.Add(14*time.Hour + 30*time.Minute)),
		Open: close, High: close + 1, Low: close - 1, Close: close, Volume: 100,
	}
}

func (test *InstrumentTests) storeBar(ctx context.Context, symbol string, day time.Time, close float64) {
	test.Require().NoError(test.assets.Store(ctx, symbol, symbol))
	at := day.Add(14*time.Hour + 30*time.Minute)
	test.Require().NoError(test.ohlc.Store(ctx, "yahoo", symbol, at, close, close+1, close-1, close, 100))
}

func (test *InstrumentTests) writeWindow(ctx context.Context, instrumentID, provider, symbol string, bars []*financepb.OHLC, windowTo *time.Time) error {
	tx, err := test.conn.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op after Commit
	if err := repository.WriteWindowMapping(ctx, tx, instrumentID, provider, symbol, bars, windowTo); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (test *InstrumentTests) mapDownloaded(ctx context.Context, instrumentID, asked, provider, symbol string, bars []*financepb.OHLC) error {
	tx, err := test.conn.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op after Commit
	if err := repository.MapDownloadedBars(ctx, tx, instrumentID, asked, provider, symbol, bars); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (test *InstrumentTests) TestBackfillWritesAnOpenRowAndClosesTheOldTicker() {
	ctx := context.Background()
	_, err := test.instruments.Store(ctx, metaISIN, "Meta")
	test.Require().NoError(err)
	_, err = test.instruments.Store(ctx, "US0000000001", "Clean")
	test.Require().NoError(err)
	test.storeSymbol(ctx, "US0000000001", "AAA", test.date(2021, 1, 4), time.Time{})
	test.storeBar(ctx, "AAA", test.date(2021, 1, 4), 10)
	test.storeBar(ctx, "AAA", test.date(2021, 1, 5), 11)

	test.storeSymbol(ctx, metaISIN, "FB", test.date(2012, 5, 18), test.date(2022, 6, 9))
	test.storeSymbol(ctx, metaISIN, "META", test.date(2022, 6, 9), time.Time{})
	test.storeBar(ctx, "FB", test.date(2022, 6, 8), 100)
	test.storeBar(ctx, "META", test.date(2022, 6, 9), 180)

	test.Require().NoError(repository.BackfillProviderSymbols(ctx, test.conn))
	test.Require().NoError(repository.BackfillProviderSymbols(ctx, test.conn))

	var aaaTo *time.Time
	var evidence string
	err = test.conn.QueryRow(ctx, `
		SELECT to_date, evidence FROM provider_symbol
		 WHERE instrument_id = $1 AND symbol = 'AAA'`, "US0000000001").Scan(&aaaTo, &evidence)
	test.Require().NoError(err)
	test.Nil(aaaTo)
	test.Equal("migration", evidence)

	var fbTo, metaTo *time.Time
	err = test.conn.QueryRow(ctx, `
		SELECT to_date FROM provider_symbol WHERE instrument_id = $1 AND symbol = 'FB'`, metaISIN).Scan(&fbTo)
	test.Require().NoError(err)
	test.Require().NotNil(fbTo)
	test.Equal("2022-06-09", fbTo.Format(time.DateOnly))
	err = test.conn.QueryRow(ctx, `
		SELECT to_date FROM provider_symbol WHERE instrument_id = $1 AND symbol = 'META'`, metaISIN).Scan(&metaTo)
	test.Require().NoError(err)
	test.Nil(metaTo)
	bars, err := test.ohlc.InstrumentBars(ctx, "yahoo", metaISIN, test.date(2022, 6, 8), test.date(2022, 6, 9))
	test.Require().NoError(err)
	test.Require().Len(bars, 2)
	test.InDelta(100, bars[0].GetClose(), 0.0001)
	test.InDelta(180, bars[1].GetClose(), 0.0001)
}

func (test *InstrumentTests) TestDownloadMappingIgnoresTheRequestEndAndRefusesASharedDay() {
	ctx := context.Background()
	_, err := test.instruments.Store(ctx, metaISIN, "Meta")
	test.Require().NoError(err)
	test.storeSymbol(ctx, metaISIN, "FB", test.date(2012, 5, 18), time.Time{})
	first := test.date(2021, 1, 4)
	later := test.date(2021, 6, 1)
	bars := []*financepb.OHLC{test.bar("META", first, 10), test.bar("META", later, 12)}
	test.Require().NoError(test.writeWindow(ctx, metaISIN, "yahoo", "META", bars[:1], nil))
	test.Require().NoError(test.writeWindow(ctx, metaISIN, "yahoo", "META", bars, nil))

	var from string
	var to *time.Time
	err = test.conn.QueryRow(ctx, `
		SELECT from_date::text, to_date FROM provider_symbol WHERE instrument_id = $1`, metaISIN).Scan(&from, &to)
	test.Require().NoError(err)
	test.Equal("2021-01-04", from)
	test.Nil(to)
	var aliases int
	err = test.conn.QueryRow(ctx, `SELECT count(*) FROM asset_symbol WHERE symbol = 'META'`).Scan(&aliases)
	test.Require().NoError(err)
	test.Zero(aliases)

	test.storeBar(ctx, "META", first, 10)
	test.storeBar(ctx, "FB", first, 9)
	err = test.writeWindow(ctx, metaISIN, "yahoo", "FB", []*financepb.OHLC{test.bar("FB", first, 9)}, nil)
	var overlap *repository.ProviderOverlapError
	test.Require().ErrorAs(err, &overlap)
	test.Equal(metaISIN, overlap.InstrumentID)
	test.Contains(overlap.Error(), "yahoo")
	test.Contains(overlap.Error(), "FB")
	test.Contains(overlap.Error(), "META")
	test.Contains(overlap.Error(), "2021-01-04")
	test.NotContains(overlap.Error(), "again")
	var symbols int
	err = test.conn.QueryRow(ctx, `SELECT count(*) FROM provider_symbol WHERE symbol = 'FB'`).Scan(&symbols)
	test.Require().NoError(err)
	test.Zero(symbols)
}

func (test *InstrumentTests) TestInstrumentBarsDoNotMixTheOtherScale() {
	ctx := context.Background()
	_, err := test.instruments.Store(ctx, metaISIN, "Meta")
	test.Require().NoError(err)
	test.storeBar(ctx, "FB", test.date(2022, 6, 8), 100)
	test.storeBar(ctx, "META", test.date(2022, 6, 7), 40)
	test.storeBar(ctx, "META", test.date(2022, 6, 9), 180)
	_, err = test.conn.Exec(ctx, `
		INSERT INTO provider_symbol (instrument_id, provider, symbol, from_date, to_date, evidence) VALUES
		($1, 'yahoo', 'FB', '2012-05-18', '2022-06-09', 'download'),
		($1, 'yahoo', 'META', '2022-06-09', NULL, 'download')`, metaISIN)
	test.Require().NoError(err)
	bars, err := test.ohlc.InstrumentBars(ctx, "yahoo", metaISIN, test.date(2022, 6, 7), test.date(2022, 6, 9))
	test.Require().NoError(err)
	test.Require().Len(bars, 2)
	test.Equal("FB", bars[0].GetSymbol())
	test.InDelta(100, bars[0].GetClose(), 0.0001)
	test.Equal("META", bars[1].GetSymbol())
	test.InDelta(180, bars[1].GetClose(), 0.0001)
}

func (test *InstrumentTests) TestLaterISINCannotOverlapTheOldFBMapping() {
	ctx := context.Background()
	_, err := test.instruments.Store(ctx, metaISIN, "Meta")
	test.Require().NoError(err)
	_, err = test.instruments.Store(ctx, "US0000000002", "Next")
	test.Require().NoError(err)
	_, err = test.conn.Exec(ctx, `
		INSERT INTO provider_symbol (instrument_id, provider, symbol, from_date, to_date, evidence)
		VALUES ($1, 'yahoo', 'FB', '2012-05-18', '2022-06-09', 'migration')`, metaISIN)
	test.Require().NoError(err)
	test.storeBar(ctx, "FB", test.date(2022, 6, 8), 100)
	before, err := test.ohlc.InstrumentBars(ctx, "yahoo", metaISIN, test.date(2022, 6, 8), test.date(2022, 6, 8))
	test.Require().NoError(err)
	test.Require().Len(before, 1)

	err = test.writeWindow(ctx, "US0000000002", "yahoo", "FB",
		[]*financepb.OHLC{test.bar("FB", test.date(2022, 6, 1), 1)}, nil)
	test.Require().Error(err)
	err = test.writeWindow(ctx, "US0000000002", "yahoo", "FB",
		[]*financepb.OHLC{test.bar("FB", test.date(2022, 6, 9), 1)}, nil)
	test.Require().NoError(err)

	after, err := test.ohlc.InstrumentBars(ctx, "yahoo", metaISIN, test.date(2022, 6, 8), test.date(2022, 6, 8))
	test.Require().NoError(err)
	test.Require().Len(after, 1)
	test.InDelta(before[0].GetClose(), after[0].GetClose(), 0.0001)
}

func (test *InstrumentTests) TestRescaleKeepsTheOldTickersReturns() {
	ctx := context.Background()
	_, err := test.instruments.Store(ctx, metaISIN, "Meta")
	test.Require().NoError(err)
	day1 := test.date(2022, 6, 7)
	day2 := test.date(2022, 6, 8)
	day3 := test.date(2022, 6, 9)
	test.storeBar(ctx, "FB", day1, 10)
	test.storeBar(ctx, "FB", day2, 11)
	test.storeBar(ctx, "META", day2, 20)
	test.storeBar(ctx, "META", day3, 22)
	_, err = test.conn.Exec(ctx, `
		INSERT INTO provider_symbol (instrument_id, provider, symbol, from_date, to_date, evidence) VALUES
		($1, 'yahoo', 'FB', '2012-05-18', '2022-06-09', 'download'),
		($1, 'yahoo', 'META', '2022-06-09', NULL, 'download')`, metaISIN)
	test.Require().NoError(err)
	// The stored META close on the comparison day moves from 20 to 10.
	err = test.ohlc.Sync(ctx, "yahoo", "META", day1, &day3, func(time.Time, *time.Time) ([]*financepb.OHLC, error) {
		return []*financepb.OHLC{test.bar("META", day2, 10), test.bar("META", day3, 11)}, nil
	})
	test.Require().NoError(err)
	var first, second float64
	err = test.conn.QueryRow(ctx, `
		SELECT close::float8 FROM ohlc WHERE symbol = 'FB' AND time::date = '2022-06-07'`).Scan(&first)
	test.Require().NoError(err)
	err = test.conn.QueryRow(ctx, `
		SELECT close::float8 FROM ohlc WHERE symbol = 'FB' AND time::date = '2022-06-08'`).Scan(&second)
	test.Require().NoError(err)
	test.InDelta(0.1, second/first-1, 0.0000001)
	test.InDelta(5, first, 0.0001)
	test.InDelta(5.5, second, 0.0001)
}

func (test *InstrumentTests) TestDuplicateDayNamesTheTickerAndTheDay() {
	day := test.date(2022, 6, 9)
	bar := test.bar("META", day, 10)
	err := repository.ValidateBars("yahoo", "META", []*financepb.OHLC{bar, bar})
	test.EqualError(err, "META has two prices on 2022-06-09")
}

func (test *InstrumentTests) TestDownloadMappingMovesTheStartEarlier() {
	ctx := context.Background()
	_, err := test.instruments.Store(ctx, metaISIN, "Meta")
	test.Require().NoError(err)
	later := test.date(2021, 6, 1)
	first := test.date(2021, 1, 4)
	test.Require().NoError(test.writeWindow(ctx, metaISIN, "yahoo", "META",
		[]*financepb.OHLC{test.bar("META", later, 12)}, nil))
	test.Require().NoError(test.writeWindow(ctx, metaISIN, "yahoo", "META",
		[]*financepb.OHLC{test.bar("META", first, 10), test.bar("META", later, 12)}, nil))
	var from string
	var n int
	err = test.conn.QueryRow(ctx, `
		SELECT count(*), min(from_date)::text FROM provider_symbol WHERE instrument_id = $1`, metaISIN).Scan(&n, &from)
	test.Require().NoError(err)
	test.Equal(1, n)
	test.Equal("2021-01-04", from)
}

func (test *InstrumentTests) TestStoredRowOnTheSameDayOutsideTheDownloadDoesNotRefuse() {
	ctx := context.Background()
	_, err := test.instruments.Store(ctx, metaISIN, "Meta")
	test.Require().NoError(err)
	test.storeSymbol(ctx, metaISIN, "META", test.date(2012, 5, 18), time.Time{})
	same := test.date(2021, 6, 1)
	// Both tickers already have a stored bar on this day. It is not a day of
	// the FB download below, so it does not refuse that download.
	test.storeBar(ctx, "META", same, 12)
	test.storeBar(ctx, "FB", same, 9)
	test.Require().NoError(test.writeWindow(ctx, metaISIN, "yahoo", "META",
		[]*financepb.OHLC{test.bar("META", same, 12)}, nil))
	err = test.writeWindow(ctx, metaISIN, "yahoo", "FB",
		[]*financepb.OHLC{test.bar("FB", test.date(2021, 8, 2), 9)}, nil)
	test.Require().NoError(err)
}

func (test *InstrumentTests) TestBackfillLeavesACrossWindowBarUnmapped() {
	ctx := context.Background()
	_, err := test.instruments.Store(ctx, metaISIN, "Meta")
	test.Require().NoError(err)
	test.storeSymbol(ctx, metaISIN, "FB", test.date(2012, 5, 18), test.date(2022, 6, 9))
	test.storeSymbol(ctx, metaISIN, "META", test.date(2022, 6, 9), time.Time{})
	test.storeBar(ctx, "FB", test.date(2022, 6, 8), 100)
	test.storeBar(ctx, "META", test.date(2022, 6, 8), 50)

	var buf bytes.Buffer
	previous := log.Logger
	log.Logger = zerolog.New(&buf)
	defer func() { log.Logger = previous }()

	test.Require().NoError(repository.BackfillProviderSymbols(ctx, test.conn))
	logged := buf.String()
	test.Contains(logged, "FB")
	test.Contains(logged, "META")
	test.Contains(logged, "2022-06-08")
	var n int
	err = test.conn.QueryRow(ctx, `SELECT count(*) FROM provider_symbol WHERE instrument_id = $1`, metaISIN).Scan(&n)
	test.Require().NoError(err)
	test.Zero(n)
}

func (test *InstrumentTests) TestBackfillLeavesAMappedYahooSeriesUnchanged() {
	ctx := context.Background()
	_, err := test.instruments.Store(ctx, metaISIN, "Meta")
	test.Require().NoError(err)
	test.storeSymbol(ctx, metaISIN, "FB", test.date(2012, 5, 18), test.date(2022, 6, 9))
	test.storeSymbol(ctx, metaISIN, "META", test.date(2022, 6, 9), time.Time{})
	// Yahoo stores the whole history under META. The days inside FB's window
	// have no FB bar.
	early := test.date(2012, 5, 18)
	later := test.date(2022, 6, 9)
	test.storeBar(ctx, "META", early, 10)
	test.storeBar(ctx, "META", later, 180)
	test.Require().NoError(test.writeWindow(ctx, metaISIN, "yahoo", "META",
		[]*financepb.OHLC{test.bar("META", early, 10), test.bar("META", later, 180)}, nil))

	var symbol, fromDate, evidence string
	var toDate *time.Time
	err = test.conn.QueryRow(ctx, `
		SELECT symbol, from_date::text, to_date, evidence
		  FROM provider_symbol WHERE instrument_id = $1`, metaISIN).Scan(&symbol, &fromDate, &toDate, &evidence)
	test.Require().NoError(err)

	var buf bytes.Buffer
	previous := log.Logger
	log.Logger = zerolog.New(&buf)
	defer func() { log.Logger = previous }()

	test.Require().NoError(repository.BackfillProviderSymbols(ctx, test.conn))
	test.NotContains(buf.String(), `"level":"error"`)

	var n int
	err = test.conn.QueryRow(ctx, `SELECT count(*) FROM provider_symbol WHERE instrument_id = $1`, metaISIN).Scan(&n)
	test.Require().NoError(err)
	test.Equal(1, n)
	var afterSymbol, afterFrom, afterEvidence string
	var afterTo *time.Time
	err = test.conn.QueryRow(ctx, `
		SELECT symbol, from_date::text, to_date, evidence
		  FROM provider_symbol WHERE instrument_id = $1`, metaISIN).Scan(&afterSymbol, &afterFrom, &afterTo, &afterEvidence)
	test.Require().NoError(err)
	test.Equal(symbol, afterSymbol)
	test.Equal(fromDate, afterFrom)
	test.Equal(toDate, afterTo)
	test.Equal(evidence, afterEvidence)
	test.Equal("META", symbol)
	test.Equal("2012-05-18", fromDate)
	test.Nil(toDate)
	test.Equal("download", evidence)
}

func (test *InstrumentTests) TestBackfillConflictLeavesThatInstrumentUnmapped() {
	ctx := context.Background()
	const next = "US0000000002"
	_, err := test.instruments.Store(ctx, metaISIN, "Meta")
	test.Require().NoError(err)
	_, err = test.instruments.Store(ctx, next, "Next")
	test.Require().NoError(err)
	_, err = test.conn.Exec(ctx, `
		INSERT INTO provider_symbol (instrument_id, provider, symbol, from_date, to_date, evidence)
		VALUES ($1, 'yahoo', 'FB', '2012-05-18', NULL, 'migration')`, metaISIN)
	test.Require().NoError(err)
	test.storeSymbol(ctx, next, "FB", test.date(2022, 6, 9), time.Time{})
	test.storeBar(ctx, "FB", test.date(2022, 6, 9), 40)

	var buf bytes.Buffer
	previous := log.Logger
	log.Logger = zerolog.New(&buf)
	defer func() { log.Logger = previous }()

	test.Require().NoError(repository.BackfillProviderSymbols(ctx, test.conn))
	logged := buf.String()
	test.Contains(logged, next)
	test.Contains(logged, "conflicted")
	var n int
	err = test.conn.QueryRow(ctx, `SELECT count(*) FROM provider_symbol WHERE instrument_id = $1`, next).Scan(&n)
	test.Require().NoError(err)
	test.Zero(n)
	err = test.conn.QueryRow(ctx, `SELECT count(*) FROM provider_symbol WHERE instrument_id = $1`, metaISIN).Scan(&n)
	test.Require().NoError(err)
	test.Equal(1, n)
}

func (test *InstrumentTests) TestBackfillKeepsEachCompanysOwnFBBars() {
	ctx := context.Background()
	const next = "US0000000002"
	_, err := test.instruments.Store(ctx, metaISIN, "Meta")
	test.Require().NoError(err)
	_, err = test.instruments.Store(ctx, next, "Next")
	test.Require().NoError(err)
	test.storeSymbol(ctx, metaISIN, "FB", test.date(2012, 5, 18), test.date(2022, 6, 9))
	test.storeSymbol(ctx, next, "FB", test.date(2022, 6, 9), time.Time{})
	test.storeBar(ctx, "FB", test.date(2022, 6, 8), 100)
	test.storeBar(ctx, "FB", test.date(2022, 6, 9), 40)
	test.Require().NoError(repository.BackfillProviderSymbols(ctx, test.conn))

	metaBars, err := test.ohlc.InstrumentBars(ctx, "yahoo", metaISIN, test.date(2022, 6, 8), test.date(2022, 6, 9))
	test.Require().NoError(err)
	test.Require().Len(metaBars, 1)
	test.InDelta(100, metaBars[0].GetClose(), 0.0001)
	nextBars, err := test.ohlc.InstrumentBars(ctx, "yahoo", next, test.date(2022, 6, 8), test.date(2022, 6, 9))
	test.Require().NoError(err)
	test.Require().Len(nextBars, 1)
	test.InDelta(40, nextBars[0].GetClose(), 0.0001)
}

func (test *InstrumentTests) TestRescaleDoesNotTouchAnotherCompanysRows() {
	ctx := context.Background()
	const next = "US0000000002"
	_, err := test.instruments.Store(ctx, metaISIN, "Meta")
	test.Require().NoError(err)
	_, err = test.instruments.Store(ctx, next, "Next")
	test.Require().NoError(err)
	test.storeBar(ctx, "META", test.date(2022, 6, 9), 40)
	test.storeBar(ctx, "META", test.date(2022, 6, 10), 44)
	test.storeBar(ctx, "FB", test.date(2022, 6, 9), 20)
	test.storeBar(ctx, "FB", test.date(2022, 6, 10), 22)
	_, err = test.conn.Exec(ctx, `
		INSERT INTO provider_symbol (instrument_id, provider, symbol, from_date, to_date, evidence) VALUES
		($1, 'yahoo', 'FB', '2012-05-18', '2022-06-09', 'download'),
		($1, 'yahoo', 'META', '2022-06-09', NULL, 'download'),
		($2, 'yahoo', 'FB', '2022-06-09', NULL, 'download')`, metaISIN, next)
	test.Require().NoError(err)
	day := test.date(2022, 6, 9)
	end := test.date(2022, 6, 10)
	err = test.ohlc.Sync(ctx, "yahoo", "FB", day, &end, func(time.Time, *time.Time) ([]*financepb.OHLC, error) {
		return []*financepb.OHLC{test.bar("FB", day, 10), test.bar("FB", end, 11)}, nil
	})
	test.Require().NoError(err)
	var first, second float64
	err = test.conn.QueryRow(ctx, `
		SELECT close::float8 FROM ohlc WHERE symbol = 'META' AND time::date = '2022-06-09'`).Scan(&first)
	test.Require().NoError(err)
	err = test.conn.QueryRow(ctx, `
		SELECT close::float8 FROM ohlc WHERE symbol = 'META' AND time::date = '2022-06-10'`).Scan(&second)
	test.Require().NoError(err)
	test.InDelta(40, first, 0.0001)
	test.InDelta(44, second, 0.0001)
}

func (test *InstrumentTests) TestRescaleOfTheOldTickerLeavesTheNewTicker() {
	ctx := context.Background()
	_, err := test.instruments.Store(ctx, metaISIN, "Meta")
	test.Require().NoError(err)
	day1 := test.date(2022, 6, 7)
	day2 := test.date(2022, 6, 8)
	day3 := test.date(2022, 6, 9)
	test.storeBar(ctx, "FB", day1, 10)
	test.storeBar(ctx, "FB", day2, 11)
	test.storeBar(ctx, "META", day3, 40)
	_, err = test.conn.Exec(ctx, `
		INSERT INTO provider_symbol (instrument_id, provider, symbol, from_date, to_date, evidence) VALUES
		($1, 'yahoo', 'FB', '2012-05-18', '2022-06-09', 'download'),
		($1, 'yahoo', 'META', '2022-06-09', NULL, 'download')`, metaISIN)
	test.Require().NoError(err)
	err = test.ohlc.Sync(ctx, "yahoo", "FB", day1, &day2, func(time.Time, *time.Time) ([]*financepb.OHLC, error) {
		return []*financepb.OHLC{test.bar("FB", day1, 5), test.bar("FB", day2, 5.5)}, nil
	})
	test.Require().NoError(err)
	var meta float64
	err = test.conn.QueryRow(ctx, `
		SELECT close::float8 FROM ohlc WHERE symbol = 'META' AND time::date = '2022-06-09'`).Scan(&meta)
	test.Require().NoError(err)
	test.InDelta(40, meta, 0.0001)
}

func (test *InstrumentTests) TestStooqSplitKeepsTheOldTickersReturnsAndTheJoin() {
	ctx := context.Background()
	_, err := test.instruments.Store(ctx, metaISIN, "Meta")
	test.Require().NoError(err)
	test.Require().NoError(test.assets.Store(ctx, "FB", "FB"))
	test.Require().NoError(test.assets.Store(ctx, "META", "META"))
	day1 := test.date(2022, 6, 7)
	day2 := test.date(2022, 6, 8)
	day3 := test.date(2022, 6, 9)
	at := func(day time.Time) time.Time { return day.Add(14*time.Hour + 30*time.Minute) }
	test.Require().NoError(test.ohlc.Store(ctx, "stooq", "FB", at(day1), 100, 101, 99, 100, 100))
	test.Require().NoError(test.ohlc.Store(ctx, "stooq", "FB", at(day2), 110, 111, 109, 110, 100))
	test.Require().NoError(test.ohlc.Store(ctx, "stooq", "META", at(day3), 121, 122, 120, 121, 100))
	_, err = test.conn.Exec(ctx, `
		INSERT INTO provider_symbol (instrument_id, provider, symbol, from_date, to_date, evidence) VALUES
		($1, 'stooq', 'FB', '2012-05-18', '2022-06-09', 'download'),
		($1, 'stooq', 'META', '2022-06-09', NULL, 'download')`, metaISIN)
	test.Require().NoError(err)
	err = test.ohlc.Sync(ctx, "stooq", "META", day1, &day3, func(time.Time, *time.Time) ([]*financepb.OHLC, error) {
		return []*financepb.OHLC{test.bar("META", day3, 60.5)}, nil
	})
	test.Require().NoError(err)
	var fb1, fb2, meta float64
	err = test.conn.QueryRow(ctx, `SELECT close::float8 FROM ohlc WHERE provider = 'stooq' AND symbol = 'FB' AND time::date = '2022-06-07'`).Scan(&fb1)
	test.Require().NoError(err)
	err = test.conn.QueryRow(ctx, `SELECT close::float8 FROM ohlc WHERE provider = 'stooq' AND symbol = 'FB' AND time::date = '2022-06-08'`).Scan(&fb2)
	test.Require().NoError(err)
	err = test.conn.QueryRow(ctx, `SELECT close::float8 FROM ohlc WHERE provider = 'stooq' AND symbol = 'META' AND time::date = '2022-06-09'`).Scan(&meta)
	test.Require().NoError(err)
	test.InDelta(0.1, fb2/fb1-1, 0.0000001)
	test.InDelta(0.1, meta/fb2-1, 0.0000001)
}

func (test *InstrumentTests) TestRefusedSyncLeavesNoMapping() {
	ctx := context.Background()
	_, err := test.instruments.Store(ctx, metaISIN, "Meta")
	test.Require().NoError(err)
	test.storeSymbol(ctx, metaISIN, "META", test.date(2012, 5, 18), time.Time{})
	test.Require().NoError(test.assets.Store(ctx, "META", "META"))
	day := test.date(2022, 6, 9)
	bar := test.bar("META", day, 10)
	err = test.ohlc.SyncSourceThen(ctx, "stooq", "META", day, &day, func(time.Time, *time.Time) ([]*financepb.OHLC, error) {
		return []*financepb.OHLC{bar}, nil
	}, nil, func(ctx context.Context, tx pgx.Tx) error {
		if err := repository.WriteWindowMapping(ctx, tx, metaISIN, "stooq", "META", []*financepb.OHLC{bar}, nil); err != nil {
			return err
		}
		return errors.New("refuse after the mapping")
	})
	test.Require().ErrorContains(err, "refuse after the mapping")
	var n int
	err = test.conn.QueryRow(ctx, `SELECT count(*) FROM provider_symbol`).Scan(&n)
	test.Require().NoError(err)
	test.Zero(n)
	err = test.conn.QueryRow(ctx, `SELECT count(*) FROM ohlc WHERE symbol = 'META'`).Scan(&n)
	test.Require().NoError(err)
	test.Zero(n)
}

func (test *InstrumentTests) TestRequestDatedAliasIsMarkedLaterTicker() {
	ctx := context.Background()
	_, err := test.instruments.Store(ctx, metaISIN, "Meta")
	test.Require().NoError(err)
	_, err = test.conn.Exec(ctx, `
		CREATE TABLE backtest (
			name text PRIMARY KEY, benchmark text NOT NULL, provider text NOT NULL DEFAULT 'yahoo');
		CREATE TABLE backtest_period (
			backtest_name text, start_date date, end_date date,
			PRIMARY KEY (backtest_name, start_date, end_date));
		INSERT INTO backtest (name, benchmark) VALUES ('demo', 'SPY');
		INSERT INTO backtest_period (backtest_name, start_date, end_date)
		VALUES ('demo', '2021-01-04', '2021-06-14');`)
	test.Require().NoError(err)
	test.storeSymbol(ctx, metaISIN, "FB", test.date(2012, 5, 18), test.date(2021, 6, 15))
	test.storeSymbol(ctx, metaISIN, "META", test.date(2021, 6, 15), time.Time{})
	test.Require().NoError(repository.BackfillProviderSymbols(ctx, test.conn))
	var fb, meta string
	err = test.conn.QueryRow(ctx, `SELECT evidence FROM asset_symbol WHERE symbol = 'FB'`).Scan(&fb)
	test.Require().NoError(err)
	err = test.conn.QueryRow(ctx, `SELECT evidence FROM asset_symbol WHERE symbol = 'META'`).Scan(&meta)
	test.Require().NoError(err)
	test.Equal("snapshot", fb)
	test.Equal("later_ticker", meta)
}

func (test *InstrumentTests) TestBarsBeforeTheFirstWindowMapToThatCompany() {
	ctx := context.Background()
	const apple = "US0378331005"
	_, err := test.instruments.Store(ctx, apple, "Apple")
	test.Require().NoError(err)
	test.storeSymbol(ctx, apple, "AAPL", test.date(2022, 12, 30), time.Time{})
	early := test.date(2021, 1, 4)
	test.Require().NoError(test.mapDownloaded(ctx, "", "AAPL", "yahoo", "AAPL", []*financepb.OHLC{
		test.bar("AAPL", early, 10), test.bar("AAPL", test.date(2023, 1, 3), 12),
	}))
	var from string
	var to *time.Time
	err = test.conn.QueryRow(ctx, `
		SELECT from_date::text, to_date FROM provider_symbol WHERE instrument_id = $1 AND symbol = 'AAPL'`, apple).
		Scan(&from, &to)
	test.Require().NoError(err)
	test.Equal("2021-01-04", from)
	test.Nil(to)
}

func (test *InstrumentTests) TestReusedTickerDoesNotReachBackPastAnotherHolder() {
	ctx := context.Background()
	const apple = "US0378331005"
	const earlier = "US0000000002"
	_, err := test.instruments.Store(ctx, earlier, "Earlier")
	test.Require().NoError(err)
	_, err = test.instruments.Store(ctx, apple, "Apple")
	test.Require().NoError(err)
	test.storeSymbol(ctx, earlier, "AAPL", test.date(2010, 1, 1), test.date(2020, 1, 1))
	test.storeSymbol(ctx, apple, "AAPL", test.date(2022, 12, 30), time.Time{})
	err = test.mapDownloaded(ctx, "", "AAPL", "yahoo", "AAPL", []*financepb.OHLC{
		test.bar("AAPL", test.date(2021, 1, 4), 10),
	})
	var uncovered *repository.UncoveredBarsError
	test.Require().ErrorAs(err, &uncovered)
	test.Require().EqualError(err, "no ticker window covers AAPL on 2021-01-04")
	var n int
	err = test.conn.QueryRow(ctx, `SELECT count(*) FROM provider_symbol`).Scan(&n)
	test.Require().NoError(err)
	test.Zero(n)
}

func (test *InstrumentTests) TestLaterTickerMappingStaysOnTheResolvedISIN() {
	ctx := context.Background()
	const next = "US0000000002"
	_, err := test.instruments.Store(ctx, metaISIN, "Meta")
	test.Require().NoError(err)
	_, err = test.instruments.Store(ctx, next, "Next")
	test.Require().NoError(err)
	test.storeSymbol(ctx, metaISIN, "FB", test.date(2019, 1, 1), test.date(2022, 6, 9))
	test.storeSymbol(ctx, next, "FB", test.date(2022, 6, 9), time.Time{})
	test.Require().NoError(test.mapDownloaded(ctx, metaISIN, "FB", "yahoo", "META", []*financepb.OHLC{
		test.bar("META", test.date(2021, 6, 1), 10),
	}))
	var id, from string
	err = test.conn.QueryRow(ctx, `
		SELECT instrument_id, from_date::text FROM provider_symbol WHERE symbol = 'META'`).Scan(&id, &from)
	test.Require().NoError(err)
	test.Equal(metaISIN, id)
	test.Equal("2021-06-01", from)

	err = test.mapDownloaded(ctx, metaISIN, "FB", "yahoo", "META", []*financepb.OHLC{
		test.bar("META", test.date(2022, 7, 1), 11),
	})
	var uncovered *repository.UncoveredBarsError
	test.Require().ErrorAs(err, &uncovered)
	test.Require().EqualError(err, "no ticker window covers META on 2022-07-01")
	var n int
	err = test.conn.QueryRow(ctx, `SELECT count(*) FROM provider_symbol WHERE instrument_id = $1`, next).Scan(&n)
	test.Require().NoError(err)
	test.Zero(n)
}

func (test *InstrumentTests) TestLaterTickerKeepsBarsBeforeTheAskedWindow() {
	ctx := context.Background()
	_, err := test.instruments.Store(ctx, metaISIN, "Meta")
	test.Require().NoError(err)
	test.storeSymbol(ctx, metaISIN, "FB", test.date(2021, 7, 1), time.Time{})
	test.Require().NoError(test.mapDownloaded(ctx, metaISIN, "FB", "yahoo", "META", []*financepb.OHLC{
		test.bar("META", test.date(2021, 1, 4), 10),
		test.bar("META", test.date(2021, 8, 2), 12),
	}))
	var from string
	var to *time.Time
	err = test.conn.QueryRow(ctx, `
		SELECT from_date::text, to_date FROM provider_symbol
		 WHERE instrument_id = $1 AND symbol = 'META'`, metaISIN).Scan(&from, &to)
	test.Require().NoError(err)
	test.Equal("2021-01-04", from)
	test.Nil(to)
}

func (test *InstrumentTests) TestLaterTickerFailsWhenAnotherCompanyHeldTheAskedTicker() {
	ctx := context.Background()
	const earlier = "US0000000002"
	_, err := test.instruments.Store(ctx, metaISIN, "Meta")
	test.Require().NoError(err)
	_, err = test.instruments.Store(ctx, earlier, "Earlier")
	test.Require().NoError(err)
	test.storeSymbol(ctx, metaISIN, "FB", test.date(2021, 7, 1), time.Time{})
	test.storeSymbol(ctx, earlier, "FB", test.date(2020, 1, 1), test.date(2021, 7, 1))
	err = test.mapDownloaded(ctx, metaISIN, "FB", "yahoo", "META", []*financepb.OHLC{
		test.bar("META", test.date(2021, 1, 4), 10),
	})
	var held *repository.HeldByError
	test.Require().ErrorAs(err, &held)
	test.Require().EqualError(err, "FB on 2021-01-04 is held by "+earlier+" and "+metaISIN)
	var n int
	err = test.conn.QueryRow(ctx, `SELECT count(*) FROM provider_symbol`).Scan(&n)
	test.Require().NoError(err)
	test.Zero(n)
}

func (test *InstrumentTests) TestPreWindowClaimNamesAnotherCompanysMapping() {
	ctx := context.Background()
	const other = "US0000000002"
	_, err := test.instruments.Store(ctx, metaISIN, "Meta")
	test.Require().NoError(err)
	_, err = test.instruments.Store(ctx, other, "Other")
	test.Require().NoError(err)
	test.storeSymbol(ctx, metaISIN, "FB", test.date(2021, 7, 1), time.Time{})
	test.storeSymbol(ctx, other, "FB", test.date(2022, 6, 9), time.Time{})
	_, err = test.conn.Exec(ctx, `
		INSERT INTO provider_symbol (instrument_id, provider, symbol, from_date, to_date, evidence)
		VALUES ($1, 'yahoo', 'FB', '2021-01-04', '2021-07-01', 'download')`, other)
	test.Require().NoError(err)
	err = test.mapDownloaded(ctx, "", "FB", "yahoo", "FB", []*financepb.OHLC{
		test.bar("FB", test.date(2021, 1, 4), 10),
	})
	var held *repository.HeldByError
	test.Require().ErrorAs(err, &held)
	test.Require().EqualError(err, "FB on 2021-01-04 is held by "+other+" and "+metaISIN)
	test.NotContains(err.Error(), "SQLSTATE")
	var n int
	err = test.conn.QueryRow(ctx, `SELECT count(*) FROM provider_symbol WHERE instrument_id = $1`, metaISIN).Scan(&n)
	test.Require().NoError(err)
	test.Zero(n)
}

func (test *InstrumentTests) TestWindowMappingClosesTheEarlierTicker() {
	ctx := context.Background()
	_, err := test.instruments.Store(ctx, metaISIN, "Meta")
	test.Require().NoError(err)
	_, err = test.conn.Exec(ctx, `
		INSERT INTO provider_symbol (instrument_id, provider, symbol, from_date, to_date, evidence)
		VALUES ($1, 'yahoo', 'FB', '2012-05-18', NULL, 'download')`, metaISIN)
	test.Require().NoError(err)
	test.Require().NoError(test.writeWindow(ctx, metaISIN, "yahoo", "META",
		[]*financepb.OHLC{test.bar("META", test.date(2021, 10, 1), 40)}, nil))
	var fbTo, metaFrom string
	var metaTo *time.Time
	err = test.conn.QueryRow(ctx, `
		SELECT to_date::text FROM provider_symbol WHERE instrument_id = $1 AND symbol = 'FB'`, metaISIN).Scan(&fbTo)
	test.Require().NoError(err)
	test.Equal("2021-10-01", fbTo)
	err = test.conn.QueryRow(ctx, `
		SELECT from_date::text, to_date FROM provider_symbol WHERE instrument_id = $1 AND symbol = 'META'`, metaISIN).
		Scan(&metaFrom, &metaTo)
	test.Require().NoError(err)
	test.Equal("2021-10-01", metaFrom)
	test.Nil(metaTo)
}

func (test *InstrumentTests) TestWindowMappingNamesAnOverlapItCannotClose() {
	ctx := context.Background()
	_, err := test.instruments.Store(ctx, metaISIN, "Meta")
	test.Require().NoError(err)
	_, err = test.conn.Exec(ctx, `
		INSERT INTO provider_symbol (instrument_id, provider, symbol, from_date, to_date, evidence)
		VALUES ($1, 'yahoo', 'FB', '2022-06-08', '2022-06-09', 'download')`, metaISIN)
	test.Require().NoError(err)
	err = test.writeWindow(ctx, metaISIN, "yahoo", "META",
		[]*financepb.OHLC{test.bar("META", test.date(2022, 6, 8), 40)}, nil)
	var overlap *repository.MappingOverlapError
	test.Require().ErrorAs(err, &overlap)
	test.Require().EqualError(err, "META overlaps FB for "+metaISIN+" on 2022-06-08")
	test.NotContains(err.Error(), "SQLSTATE")
	var n int
	err = test.conn.QueryRow(ctx, `SELECT count(*) FROM provider_symbol WHERE symbol = 'META'`).Scan(&n)
	test.Require().NoError(err)
	test.Zero(n)
}
