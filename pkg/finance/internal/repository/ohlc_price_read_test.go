package repository_test

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/lhjnilsson/foreverbull/pkg/finance/internal/repository"
)

// insertStoredBar writes one ohlc row with SQL so Store's price check does
// not run. Prices are numeric text: "0", "-1", "NaN", "Infinity".
func (test *OHLCTests) insertStoredBar(symbol string, day time.Time, open, high, low, close string) {
	test.T().Helper()
	_, err := test.conn.Exec(context.Background(), `
		INSERT INTO ohlc (provider, symbol, time, open, high, low, close, volume)
		VALUES ('yahoo', $1, $2, $3::numeric, $4::numeric, $5::numeric, $6::numeric, 10)`,
		symbol, day, open, high, low, close)
	test.Require().NoError(err)
}

// storedOHLCText is every stored bar, so a read can be shown to leave the
// table unchanged.
func (test *OHLCTests) storedOHLCText() []string {
	test.T().Helper()
	rows, err := test.conn.Query(context.Background(), `
		SELECT symbol, time, open::text, high::text, low::text, close::text, volume::text
		FROM ohlc ORDER BY symbol, time`)
	test.Require().NoError(err)
	defer rows.Close()
	var got []string
	for rows.Next() {
		var symbol, open, high, low, close, volume string
		var at time.Time
		test.Require().NoError(rows.Scan(&symbol, &at, &open, &high, &low, &close, &volume))
		got = append(got, fmt.Sprintf("%s %s %s %s %s %s %s",
			symbol, at.UTC().Format("2006-01-02T15:04:05"), open, high, low, close, volume))
	}
	test.Require().NoError(rows.Err())
	return got
}

func (test *OHLCTests) TestListOmitsStoredZeroClose() {
	ctx := context.Background()
	good := time.Date(2020, 4, 1, 0, 0, 0, 0, time.UTC)
	zero := good.AddDate(0, 0, 1)
	test.insertStoredBar("ABC", good, "1.2", "1.3", "1.1", "10")
	test.insertStoredBar("ABC", zero, "1.2", "1.3", "1.1", "0")
	stored := test.storedOHLCText()

	bars, err := test.ohlcStorage.List(ctx, "yahoo", []string{"ABC"}, zero, 10)
	test.Require().NoError(err)
	test.Require().Len(bars, 1)
	test.Equal("ABC", bars[0].GetSymbol())
	test.InDelta(10, bars[0].GetClose(), 0.0001)
	test.Equal("2020-04-01", bars[0].GetTimestamp().AsTime().UTC().Format("2006-01-02"))
	test.Equal(stored, test.storedOHLCText())
}

func (test *OHLCTests) TestListOmitsStoredNegativeAndNaN() {
	ctx := context.Background()
	day := time.Date(2020, 5, 1, 0, 0, 0, 0, time.UTC)
	test.insertStoredBar("ABC", day, "1", "2", "0.5", "-1")
	test.insertStoredBar("ABC", day.AddDate(0, 0, 1), "1", "2", "0.5", "NaN")
	test.insertStoredBar("ABC", day.AddDate(0, 0, 2), "1", "2", "0.5", "Infinity")
	test.insertStoredBar("ABC", day.AddDate(0, 0, 3), "1", "2", "0.5", "-Infinity")
	test.insertStoredBar("ABC", day.AddDate(0, 0, 4), "Infinity", "2", "0.5", "8")
	test.insertStoredBar("ABC", day.AddDate(0, 0, 5), "NaN", "2", "0.5", "9")
	test.insertStoredBar("ABC", day.AddDate(0, 0, 6), "1", "2", "0.5", "10")
	stored := test.storedOHLCText()
	test.Require().Len(stored, 7)

	bars, err := test.ohlcStorage.List(ctx, "yahoo", []string{"ABC"}, day.AddDate(0, 0, 6), 20)
	test.Require().NoError(err)
	test.Require().Len(bars, 1)
	test.InDelta(10, bars[0].GetClose(), 0.0001)
	test.Equal("2020-05-07", bars[0].GetTimestamp().AsTime().UTC().Format("2006-01-02"))
	test.Equal(stored, test.storedOHLCText())
}

func (test *OHLCTests) TestListResolvedOmitsStoredBadBar() {
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

	good := time.Date(2021, 11, 1, 21, 0, 0, 0, time.UTC)
	directBad := time.Date(2021, 11, 2, 21, 0, 0, 0, time.UTC)
	aliasBad := time.Date(2021, 11, 3, 21, 0, 0, 0, time.UTC)
	test.insertStoredBar("META", good, "1", "2", "0.5", "20")
	test.insertStoredBar("META", directBad, "1", "2", "0.5", "0")
	test.insertStoredBar("FB", aliasBad, "1", "2", "0.5", "NaN")
	stored := test.storedOHLCText()

	asOf := time.Date(2021, 11, 15, 21, 0, 0, 0, time.UTC)
	bars, err := test.ohlcStorage.ListResolved(ctx, "yahoo", []string{"META"}, asOf, 1)
	test.Require().NoError(err)
	test.Require().Len(bars, 1)
	test.Equal("META", bars[0].GetSymbol())
	test.InDelta(20, bars[0].GetClose(), 0.0001)
	test.Equal("2021-11-01", bars[0].GetTimestamp().AsTime().UTC().Format("2006-01-02"))
	test.Equal(stored, test.storedOHLCText())
}

func (test *OHLCTests) TestCloseTreatsZeroCloseAsMissing() {
	ctx := context.Background()
	zeroDay := time.Date(2020, 3, 2, 15, 0, 0, 0, time.UTC)
	nanDay := zeroDay.AddDate(0, 0, 1)
	infDay := zeroDay.AddDate(0, 0, 2)
	goodDay := zeroDay.AddDate(0, 0, 3)
	missingDay := zeroDay.AddDate(0, 0, 4)
	test.insertStoredBar("ABC", zeroDay, "1", "2", "0.5", "0")
	test.insertStoredBar("ABC", nanDay, "1", "2", "0.5", "NaN")
	test.insertStoredBar("ABC", infDay, "1", "2", "0.5", "Infinity")
	test.insertStoredBar("ABC", goodDay, "1", "2", "0.5", "11")
	stored := test.storedOHLCText()

	missingPx, missingErr := test.ohlcStorage.Close(ctx, "yahoo", "ABC", missingDay)
	test.Require().ErrorIs(missingErr, pgx.ErrNoRows)

	for _, day := range []time.Time{zeroDay, nanDay, infDay} {
		name := day.UTC().Format("2006-01-02")
		px, err := test.ohlcStorage.Close(ctx, "yahoo", "ABC", day)
		test.Require().ErrorIs(err, pgx.ErrNoRows, name)
		test.Equal(missingErr.Error(), err.Error(), name)
		test.InDelta(missingPx, px, 0, name)
	}

	good, err := test.ohlcStorage.Close(ctx, "yahoo", "ABC", goodDay)
	test.Require().NoError(err)
	test.InDelta(11, good, 0.0001)
	test.Equal(stored, test.storedOHLCText())
}

// A non-positive open, high, or low leaves the row out of List even when
// the close itself is a real price. Each bad row stays stored.
func (test *OHLCTests) TestListOmitsBadOpenHighAndLow() {
	ctx := context.Background()
	day := time.Date(2020, 7, 1, 0, 0, 0, 0, time.UTC)
	test.insertStoredBar("ABC", day, "1", "2", "0.5", "10")
	test.insertStoredBar("ABC", day.AddDate(0, 0, 1), "0", "2", "0.5", "11")
	test.insertStoredBar("ABC", day.AddDate(0, 0, 2), "-1", "2", "0.5", "12")
	test.insertStoredBar("ABC", day.AddDate(0, 0, 3), "1", "0", "0", "13")
	test.insertStoredBar("ABC", day.AddDate(0, 0, 4), "1", "2", "-1", "14")
	stored := test.storedOHLCText()
	test.Require().Len(stored, 5)

	bars, err := test.ohlcStorage.List(ctx, "yahoo", []string{"ABC"}, day.AddDate(0, 0, 4), 10)
	test.Require().NoError(err)
	test.Require().Len(bars, 1)
	test.InDelta(10, bars[0].GetClose(), 0.0001)
	test.Equal("2020-07-01", bars[0].GetTimestamp().AsTime().UTC().Format("2006-01-02"))
	test.Equal(stored, test.storedOHLCText())
}

// The in-range ticker and the newest timestamp are preferred, but the price
// filter runs before DISTINCT ON and before ROW_NUMBER. A bad preferred row
// does not hide the good row for that day, and it stays stored.
func (test *OHLCTests) TestPreferredBadRowDoesNotHideTheGoodBar() {
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

	// FB's dates cover 2021-10-01, so DISTINCT ON prefers that row. Its
	// close is 0. META at the same time is the fallback and is a real bar.
	at := time.Date(2021, 10, 1, 21, 0, 0, 0, time.UTC)
	test.insertStoredBar("FB", at, "1", "2", "0.5", "0")
	test.insertStoredBar("META", at, "1", "2", "0.5", "20")

	// Same calendar day, two ABC times. ROW_NUMBER prefers the later time.
	// That row's close is 0; the earlier row is the bar List must return.
	morning := time.Date(2020, 7, 6, 14, 0, 0, 0, time.UTC)
	later := time.Date(2020, 7, 6, 20, 0, 0, 0, time.UTC)
	test.insertStoredBar("ABC", morning, "1", "2", "0.5", "11")
	test.insertStoredBar("ABC", later, "1", "2", "0.5", "0")
	stored := test.storedOHLCText()

	resolved, err := test.ohlcStorage.ListResolved(ctx, "yahoo", []string{"META"}, at, 10)
	test.Require().NoError(err)
	test.Require().Len(resolved, 1)
	test.Equal("META", resolved[0].GetSymbol())
	test.InDelta(20, resolved[0].GetClose(), 0.0001)

	bars, err := test.ohlcStorage.List(ctx, "yahoo", []string{"ABC"}, later, 1)
	test.Require().NoError(err)
	test.Require().Len(bars, 1)
	test.InDelta(11, bars[0].GetClose(), 0.0001)
	test.Equal("2020-07-06T14:00:00", bars[0].GetTimestamp().AsTime().UTC().Format("2006-01-02T15:04:05"))
	test.Equal(stored, test.storedOHLCText())
}

func (test *OHLCTests) TestListLimitCountsOnlyValidBars() {
	ctx := context.Background()
	day := time.Date(2020, 6, 1, 0, 0, 0, 0, time.UTC)
	// Newest raw window is a NaN, then 14, then 13. The zero and the NaN
	// must not consume a slot, so limit 3 is the three valid closes.
	for i, close := range []string{"10", "11", "0", "13", "14", "NaN"} {
		test.insertStoredBar("ABC", day.AddDate(0, 0, i), "1", "2", "0.5", close)
	}
	stored := test.storedOHLCText()

	bars, err := test.ohlcStorage.List(ctx, "yahoo", []string{"ABC"}, day.AddDate(0, 0, 5), 3)
	test.Require().NoError(err)
	test.Require().Len(bars, 3)
	test.InDelta(11, bars[0].GetClose(), 0.0001)
	test.InDelta(13, bars[1].GetClose(), 0.0001)
	test.InDelta(14, bars[2].GetClose(), 0.0001)
	test.Equal(stored, test.storedOHLCText())
}
