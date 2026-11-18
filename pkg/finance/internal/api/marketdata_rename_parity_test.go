package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/internal/test_helper"
	financebars "github.com/lhjnilsson/foreverbull/pkg/finance/bars"
	"github.com/lhjnilsson/foreverbull/pkg/finance/internal/repository"
	"github.com/lhjnilsson/foreverbull/pkg/finance/internal/suppliers/marketdata"
	"github.com/lhjnilsson/foreverbull/pkg/finance/supplier"
	pb "github.com/lhjnilsson/foreverbull/pkg/pb/finance"
	servicePb "github.com/lhjnilsson/foreverbull/pkg/pb/finance/service"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// renameMetaISIN is Meta. otherISIN reuses FB after Meta has moved to META.
// The two windows do not overlap, so a date picks one instrument.
const (
	renameMetaISIN = "US30303M1027"
	otherISIN      = "US0000000002"
)

// renameQuote is one daily bar a fake Stooq or Alpaca feed returns.
// The four closes are distinct so a bar that landed on the wrong
// instrument is visible.
type renameQuote struct {
	day                string
	open, high, low, c float64
	volume             uint64
}

var (
	// Meta's October bar is before the 2021-10-31 rename. Yahoo serves it
	// under META; a provider that keeps the ticker of the day serves it
	// under FB.
	metaBefore = renameQuote{"2021-10-01", 180, 181, 179, 180.5, 1001}
	metaAfter  = renameQuote{"2021-11-01", 220, 221, 219, 220.5, 1002}
	// The other company's bar, stored under FB once that company holds it.
	reusedFB = renameQuote{"2022-06-02", 999, 1000, 998, 999.5, 1003}
	// The other company's bar from before it took FB.
	otherOld = renameQuote{"2022-05-03", 50, 51, 49, 50.5, 1004}
)

func renameSpan() (time.Time, time.Time) {
	return time.Date(2021, 9, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2022, 6, 30, 0, 0, 0, 0, time.UTC)
}

func stooqCSV(quotes []renameQuote) string {
	var b strings.Builder
	b.WriteString("Date,Open,High,Low,Close,Volume\n")
	for _, q := range quotes {
		fmt.Fprintf(&b, "%s,%.1f,%.1f,%.1f,%.1f,%d\n", q.day, q.open, q.high, q.low, q.c, q.volume)
	}
	return b.String()
}

func alpacaBarsJSON(bySymbol map[string][]renameQuote) string {
	bars := make(map[string][]map[string]any, len(bySymbol))
	for symbol, quotes := range bySymbol {
		rows := make([]map[string]any, 0, len(quotes))
		for _, q := range quotes {
			rows = append(rows, map[string]any{
				"t": q.day + "T00:00:00Z",
				"o": q.open, "h": q.high, "l": q.low, "c": q.c, "v": q.volume,
			})
		}
		bars[symbol] = rows
	}
	body, err := json.Marshal(map[string]any{"bars": bars, "next_page_token": nil})
	if err != nil {
		panic(err)
	}
	return string(body)
}

const alpacaEmptyActions = `{"corporate_actions":{},"next_page_token":null}`

// stooqRenameFeed serves daily CSV, or Stooq's "No data" when a code is
// absent. Codes are the US form the client asks for (fb.us).
func stooqRenameFeed(t *testing.T, csvByCode map[string]string) (*httptest.Server, supplier.Marketdata) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, ok := csvByCode[r.URL.Query().Get("s")]
		if !ok {
			_, _ = w.Write([]byte("No data\n"))
			return
		}
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	client, err := marketdata.NewStooqClient(
		marketdata.WithStooqExchange("US"),
		marketdata.WithStooqBaseURL(srv.URL),
	)
	require.NoError(t, err)
	return srv, client
}

// alpacaRenameFeed serves split-style bars and an empty corporate-action
// list, so a download can store prices without a dividend.
func alpacaRenameFeed(t *testing.T, quotes map[string][]renameQuote) supplier.Marketdata {
	t.Helper()
	body := alpacaBarsJSON(quotes)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "corporate-actions") {
			_, _ = w.Write([]byte(alpacaEmptyActions))
			return
		}
		if !strings.Contains(r.URL.Path, "/bars") {
			http.Error(w, "unexpected path "+r.URL.Path, http.StatusNotFound)
			return
		}
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	client, err := marketdata.NewAlpacaBars("key", "secret", marketdata.WithAlpacaBaseURL(srv.URL))
	require.NoError(t, err)
	return client
}

func requireFeedQuotes(t *testing.T, bars []*pb.OHLC, quotes []renameQuote) {
	t.Helper()
	require.Len(t, bars, len(quotes))
	for i, q := range quotes {
		require.Equal(t, q.day, bars[i].GetTimestamp().AsTime().UTC().Format(time.DateOnly))
		require.InDelta(t, q.open, bars[i].GetOpen(), 1e-9)
		require.InDelta(t, q.high, bars[i].GetHigh(), 1e-9)
		require.InDelta(t, q.low, bars[i].GetLow(), 1e-9)
		require.InDelta(t, q.c, bars[i].GetClose(), 1e-9)
		require.Equal(t, q.volume, bars[i].GetVolume())
	}
}

// The feeds keep each ticker's OHLC. Stooq's "No data" and Alpaca's empty
// bar list are the two answers a later-ticker lookup treats as "this
// ticker has none", the same as Yahoo's Not Found.
func TestStooqAndAlpacaRenameFeedsKeepEachTickersOHLC(t *testing.T) {
	start, end := renameSpan()
	split := map[string][]renameQuote{
		"FB":   {metaBefore, reusedFB},
		"META": {metaAfter},
		"NNN":  {otherOld},
	}
	_, stooqSplit := stooqRenameFeed(t, map[string]string{
		"fb.us":   stooqCSV(split["FB"]),
		"meta.us": stooqCSV(split["META"]),
		"nnn.us":  stooqCSV(split["NNN"]),
	})
	alpacaSplit := alpacaRenameFeed(t, split)
	for _, symbol := range []string{"FB", "META", "NNN"} {
		stooqBars, err := stooqSplit.GetOHLC(context.Background(), symbol, start, &end)
		require.NoError(t, err, symbol)
		for _, bar := range stooqBars {
			require.Equal(t, symbol, bar.GetSymbol())
		}
		requireFeedQuotes(t, stooqBars, split[symbol])

		alpacaBars, err := alpacaSplit.GetOHLC(context.Background(), symbol, start, &end)
		require.NoError(t, err, symbol)
		requireFeedQuotes(t, alpacaBars, split[symbol])
	}

	_, stooqNone := stooqRenameFeed(t, map[string]string{
		"meta.us": stooqCSV([]renameQuote{metaBefore, metaAfter}),
	})
	_, err := stooqNone.GetOHLC(context.Background(), "FB", start, &end)
	require.ErrorIs(t, err, marketdata.ErrNoBars)
	metaBars, err := stooqNone.GetOHLC(context.Background(), "META", start, &end)
	require.NoError(t, err)
	requireFeedQuotes(t, metaBars, []renameQuote{metaBefore, metaAfter})

	alpacaNone := alpacaRenameFeed(t, map[string][]renameQuote{
		"META": {metaBefore, metaAfter},
	})
	empty, err := alpacaNone.GetOHLC(context.Background(), "FB", start, &end)
	require.NoError(t, err)
	require.Empty(t, empty)
	metaBars, err = alpacaNone.GetOHLC(context.Background(), "META", start, &end)
	require.NoError(t, err)
	requireFeedQuotes(t, metaBars, []renameQuote{metaBefore, metaAfter})
}

func renamePool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	test_helper.SetupEnvironment(t, &test_helper.Containers{Postgres: true})
	pool, err := pgxpool.New(context.Background(), environment.GetPostgresURL())
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	require.NoError(t, repository.Recreate(context.Background(), pool))
	return pool
}

func seedRename(t *testing.T, pool *pgxpool.Pool, other bool) {
	t.Helper()
	_, err := pool.Exec(context.Background(), `
		INSERT INTO instrument (id, isin, name) VALUES
			($1, $1, 'Meta Platforms'),
			($2, $2, 'Other Corp');
		INSERT INTO asset_symbol (isin, symbol, from_date, to_date) VALUES
			($1, 'FB', '2021-07-01', '2021-10-31'),
			($1, 'META', '2021-10-31', NULL)`,
		renameMetaISIN, otherISIN)
	require.NoError(t, err)
	if !other {
		return
	}
	_, err = pool.Exec(context.Background(), `
		INSERT INTO asset_symbol (isin, symbol, from_date, to_date) VALUES
			($1, 'NNN', '2021-07-01', '2022-06-01'),
			($1, 'FB', '2022-06-01', NULL)`, otherISIN)
	require.NoError(t, err)
}

func renameMetadata(t *testing.T) *supplier.MockMarketdata {
	t.Helper()
	md := supplier.NewMockMarketdata(t)
	for symbol, name := range map[string]string{
		"FB": "Facebook", "META": "Meta Platforms", "NNN": "Other Corp",
	} {
		md.On("GetAsset", mock.Anything, symbol).Return(&pb.Asset{Symbol: symbol, Name: name}, nil).Maybe()
	}
	md.On("SearchAsset", mock.Anything, renameMetaISIN).Return([]*pb.Asset{{
		Symbol: "META", Name: "Meta Platforms", ISIN: renameMetaISIN,
	}}, nil).Maybe()
	return md
}

func downloadRename(t *testing.T, pool *pgxpool.Pool, provider, symbol string, bars supplier.Marketdata) *servicePb.DownloadHistoricalDataResponse {
	t.Helper()
	start, end := renameSpan()
	stream := &countingStream{}
	blocked := NewMarketdataServer(pool, renameMetadata(t)).downloadSymbol(
		context.Background(), newDownloadSender(stream, func() {}),
		symbol, start, &end, provider,
		repository.Asset{Conn: pool}, repository.OHLC{Conn: pool}, bars,
	)
	require.Nil(t, blocked)
	require.Len(t, stream.sent, 1)
	msg := stream.sent[0]
	require.Equalf(t, servicePb.DownloadHistoricalDataResponse_STATUS_OK, msg.GetStatus(),
		"%s %s: %s", provider, symbol, msg.GetError())
	require.Equal(t, symbol, msg.GetSymbol())
	return msg
}

func storedCloses(t *testing.T, pool *pgxpool.Pool, provider, symbol string) map[string]float64 {
	t.Helper()
	rows, err := pool.Query(context.Background(), `
		SELECT time, close::float8
		  FROM ohlc
		 WHERE provider = $1 AND symbol = $2
		 ORDER BY time`, provider, symbol)
	require.NoError(t, err)
	defer rows.Close()
	got := map[string]float64{}
	for rows.Next() {
		var at time.Time
		var closePx float64
		require.NoError(t, rows.Scan(&at, &closePx))
		got[at.UTC().Format(time.DateOnly)] = closePx
	}
	require.NoError(t, rows.Err())
	return got
}

func requireResolved(t *testing.T, pool *pgxpool.Pool, provider, symbol string, asOf time.Time, quotes []renameQuote) {
	t.Helper()
	bars, err := financebars.New(pool).List(context.Background(), provider, []string{symbol}, asOf, 20)
	require.NoError(t, err, provider+" "+symbol)
	require.Len(t, bars, len(quotes), provider+" "+symbol)
	for i, q := range quotes {
		require.Equal(t, symbol, bars[i].GetSymbol(), "%s bar %d", provider, i)
		require.Equal(t, q.day, bars[i].GetTimestamp().AsTime().UTC().Format(time.DateOnly), provider)
		require.InDelta(t, q.c, bars[i].GetClose(), 1e-9, "%s %s", provider, q.day)
		require.InDelta(t, q.open, bars[i].GetOpen(), 1e-9, provider)
		require.Equal(t, q.volume, bars[i].GetVolume(), provider)
	}
}

func requireSymbolAt(t *testing.T, pool *pgxpool.Pool, isin string, day time.Time, symbol string) {
	t.Helper()
	instruments := repository.Instruments{Conn: pool}
	got, err := instruments.Symbols(context.Background(), &repository.Instrument{ISIN: isin}, day)
	require.NoError(t, err)
	require.Equal(t, symbol, got, day.Format(time.DateOnly))
	resolved, err := instruments.Resolve(context.Background(), symbol, day)
	require.NoError(t, err)
	require.Equal(t, isin, resolved.ISIN)
}

// Split history is the layout universe.md allows besides Yahoo's: FB up to
// the rename, META from it, and a later company that takes the freed ticker.
// Each company's bars stay on its ISIN. The name on a date is the ticker
// asset_symbol gives that instrument, which is the name a held position uses.
func TestStooqAndAlpacaSplitRenameStaysOnTheISIN(t *testing.T) {
	quotes := map[string][]renameQuote{
		"FB":   {metaBefore, reusedFB},
		"META": {metaAfter},
		"NNN":  {otherOld},
	}
	feeds := map[string]supplier.Marketdata{
		"stooq": func() supplier.Marketdata {
			_, client := stooqRenameFeed(t, map[string]string{
				"fb.us":   stooqCSV(quotes["FB"]),
				"meta.us": stooqCSV(quotes["META"]),
				"nnn.us":  stooqCSV(quotes["NNN"]),
			})
			return client
		}(),
		"alpaca": alpacaRenameFeed(t, quotes),
	}
	asOf := time.Date(2022, 7, 15, 21, 0, 0, 0, time.UTC)
	for _, provider := range []string{"stooq", "alpaca"} {
		t.Run(provider, func(t *testing.T) {
			pool := renamePool(t)
			seedRename(t, pool, true)
			for _, symbol := range []string{"FB", "META", "NNN"} {
				msg := downloadRename(t, pool, provider, symbol, feeds[provider])
				require.Nil(t, msg.GetLaterTicker(), symbol)
			}
			require.Equal(t, map[string]float64{
				metaBefore.day: metaBefore.c, reusedFB.day: reusedFB.c,
			}, storedCloses(t, pool, provider, "FB"))
			require.Equal(t, map[string]float64{metaAfter.day: metaAfter.c}, storedCloses(t, pool, provider, "META"))
			require.Equal(t, map[string]float64{otherOld.day: otherOld.c}, storedCloses(t, pool, provider, "NNN"))

			// Copy onto yahoo so the three providers are read the same way.
			_, err := pool.Exec(context.Background(), `
				INSERT INTO ohlc (symbol, open, high, low, close, volume, time, provider)
				SELECT symbol, open, high, low, close, volume, time, 'yahoo'
				  FROM ohlc WHERE provider = $1`, provider)
			require.NoError(t, err)

			// asOf is after both renames. META is Meta's history, including the
			// October bar stored under FB. FB is the other company's history,
			// including the May bar stored under NNN. Neither gets the other's bar.
			oct15 := time.Date(2021, 10, 15, 21, 0, 0, 0, time.UTC)
			may15 := time.Date(2022, 5, 15, 21, 0, 0, 0, time.UTC)
			june15 := time.Date(2022, 6, 15, 21, 0, 0, 0, time.UTC)
			for _, name := range []string{provider, "yahoo"} {
				requireResolved(t, pool, name, "META", asOf, []renameQuote{metaBefore, metaAfter})
				requireResolved(t, pool, name, "FB", oct15, []renameQuote{metaBefore})
				requireResolved(t, pool, name, "FB", june15, []renameQuote{otherOld, reusedFB})
				requireResolved(t, pool, name, "NNN", may15, []renameQuote{otherOld})
			}
			requireSymbolAt(t, pool, renameMetaISIN, time.Date(2021, 10, 15, 0, 0, 0, 0, time.UTC), "FB")
			requireSymbolAt(t, pool, renameMetaISIN, time.Date(2021, 11, 1, 0, 0, 0, 0, time.UTC), "META")
			requireSymbolAt(t, pool, otherISIN, time.Date(2022, 5, 3, 0, 0, 0, 0, time.UTC), "NNN")
			requireSymbolAt(t, pool, otherISIN, time.Date(2022, 6, 2, 0, 0, 0, 0, time.UTC), "FB")
		})
	}
}

// Yahoo keeps a renamed company's whole history under the current ticker
// and has nothing under the old one. Stooq answers that with "No data" and
// Alpaca with an empty bar list. The download stores the later ticker's
// bars, and a read of FB before the rename is those prices under FB.
func TestStooqAndAlpacaCurrentTickerCoversTheRename(t *testing.T) {
	quotes := map[string][]renameQuote{"META": {metaBefore, metaAfter}}
	feeds := map[string]supplier.Marketdata{
		"stooq": func() supplier.Marketdata {
			_, client := stooqRenameFeed(t, map[string]string{
				"meta.us": stooqCSV(quotes["META"]),
			})
			return client
		}(),
		"alpaca": alpacaRenameFeed(t, quotes),
	}
	before := time.Date(2021, 10, 15, 21, 0, 0, 0, time.UTC)
	after := time.Date(2021, 11, 15, 21, 0, 0, 0, time.UTC)
	for _, provider := range []string{"stooq", "alpaca"} {
		t.Run(provider, func(t *testing.T) {
			pool := renamePool(t)
			seedRename(t, pool, false)
			msg := downloadRename(t, pool, provider, "FB", feeds[provider])
			require.NotNil(t, msg.GetLaterTicker())
			require.Equal(t, "META", msg.GetLaterTicker().GetSymbol())
			require.Empty(t, storedCloses(t, pool, provider, "FB"))
			require.Equal(t, map[string]float64{
				metaBefore.day: metaBefore.c, metaAfter.day: metaAfter.c,
			}, storedCloses(t, pool, provider, "META"))

			_, err := pool.Exec(context.Background(), `
				INSERT INTO ohlc (symbol, open, high, low, close, volume, time, provider)
				SELECT symbol, open, high, low, close, volume, time, 'yahoo'
				  FROM ohlc WHERE provider = $1`, provider)
			require.NoError(t, err)
			for _, name := range []string{provider, "yahoo"} {
				requireResolved(t, pool, name, "FB", before, []renameQuote{metaBefore})
				requireResolved(t, pool, name, "META", after, []renameQuote{metaBefore, metaAfter})
			}
			requireSymbolAt(t, pool, renameMetaISIN, time.Date(2021, 10, 15, 0, 0, 0, 0, time.UTC), "FB")
			requireSymbolAt(t, pool, renameMetaISIN, time.Date(2021, 11, 1, 0, 0, 0, 0, time.UTC), "META")
		})
	}
}
