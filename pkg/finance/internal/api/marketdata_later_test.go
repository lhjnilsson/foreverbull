package api

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/internal/test_helper"
	"github.com/lhjnilsson/foreverbull/pkg/finance/internal/repository"
	"github.com/lhjnilsson/foreverbull/pkg/finance/internal/suppliers/marketdata"
	"github.com/lhjnilsson/foreverbull/pkg/finance/supplier"
	pb "github.com/lhjnilsson/foreverbull/pkg/pb/finance"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// A download cancelled while a later ticker is fetched returns the context
// error, not "no price data", even when GetOHLC answered with no bars.
func TestFetchBarsViaLaterTickerReturnsContextErrorWhenCancelled(t *testing.T) {
	test_helper.SetupEnvironment(t, &test_helper.Containers{Postgres: true})
	pool, err := pgxpool.New(context.Background(), environment.GetPostgresURL())
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	require.NoError(t, repository.Recreate(context.Background(), pool))

	const isin = "US30303M1027"
	instruments := repository.Instruments{Conn: pool}
	_, err = instruments.Store(context.Background(), isin, "Facebook Inc")
	require.NoError(t, err)
	_, err = pool.Exec(context.Background(), `
		INSERT INTO asset_symbol (isin, symbol, from_date, to_date)
		VALUES ($1, 'FB', '2019-01-01'::date, NULL)`, isin)
	require.NoError(t, err)

	md := supplier.NewMockMarketdata(t)
	md.On("SearchAsset", mock.Anything, isin).Return([]*pb.Asset{{Symbol: "META", ISIN: isin}}, nil)
	fs := NewMarketdataServer(pool, dividendFree{md})

	start := time.Date(2019, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2020, 12, 31, 0, 0, 0, 0, time.UTC)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	md.On("GetOHLC", mock.Anything, "META", mock.Anything, mock.Anything).Run(func(mock.Arguments) { cancel() }).
		Return(nil, fmt.Errorf("fail to get OHLC data for symbol META: %w", marketdata.ErrNoBars)).Once()

	fetched, err := fs.fetchBarsViaLaterTicker(ctx, "FB", start, &end, "yahoo", "", md, repository.Asset{Conn: pool})
	require.Nil(t, fetched)
	require.ErrorIs(t, err, context.Canceled)
	require.NotErrorIs(t, err, errNoPriceData)
	md.AssertNotCalled(t, "GetAsset", mock.Anything, "META")
}

func TestFetchBarsViaLaterTickerRefusesALondonCandidate(t *testing.T) {
	test_helper.SetupEnvironment(t, &test_helper.Containers{Postgres: true})
	pool, err := pgxpool.New(context.Background(), environment.GetPostgresURL())
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	require.NoError(t, repository.Recreate(context.Background(), pool))

	const isin = "GB00BH4HKS39"
	instruments := repository.Instruments{Conn: pool}
	_, err = instruments.Store(context.Background(), isin, "Vodafone")
	require.NoError(t, err)
	_, err = pool.Exec(context.Background(), `
		INSERT INTO asset_symbol (isin, symbol, from_date, to_date)
		VALUES ($1, 'VOD', '2012-01-01'::date, '2020-06-01'::date),
		       ($1, 'VOD.L', '2020-06-01'::date, NULL)`, isin)
	require.NoError(t, err)

	restore := marketdata.SetQuoteReader(func(_ context.Context, _, _, symbol string) (marketdata.Quote, error) {
		if symbol == "VOD.L" {
			return marketdata.Quote{Exchange: "LSE", Currency: "GBP", Reported: true}, nil
		}
		return marketdata.Quote{Exchange: "NMS", Currency: "USD", Reported: true}, nil
	})
	t.Cleanup(restore)

	md := supplier.NewMockMarketdata(t)
	md.On("SearchAsset", mock.Anything, isin).Return([]*pb.Asset{}, nil)
	fs := NewMarketdataServer(pool, dividendFree{md})
	start := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2020, 12, 31, 0, 0, 0, 0, time.UTC)
	fetched, err := fs.fetchBarsViaLaterTicker(context.Background(), "VOD", start, &end, "yahoo", "", md, repository.Asset{Conn: pool})
	require.Nil(t, fetched)
	require.ErrorIs(t, err, errRefusedCandidate)
	require.EqualError(t, err, "Prices for VOD come from VOD.L, the company's later ticker, which is listed outside the US (London, GBP). Global markets aren't supported yet.")
	md.AssertNotCalled(t, "GetOHLC", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	md.AssertNotCalled(t, "GetAsset", mock.Anything, mock.Anything)
	var bars int
	require.NoError(t, pool.QueryRow(context.Background(), `SELECT count(*) FROM ohlc`).Scan(&bars))
	require.Zero(t, bars)
}

func TestDownloadHistoricalDataNamesARefusedLaterTicker(t *testing.T) {
	test_helper.SetupEnvironment(t, &test_helper.Containers{Postgres: true})
	pool, err := pgxpool.New(context.Background(), environment.GetPostgresURL())
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	require.NoError(t, repository.Recreate(context.Background(), pool))

	const isin = "GB00BH4HKS39"
	instruments := repository.Instruments{Conn: pool}
	_, err = instruments.Store(context.Background(), isin, "Vodafone")
	require.NoError(t, err)
	_, err = pool.Exec(context.Background(), `
		INSERT INTO asset_symbol (isin, symbol, from_date, to_date)
		VALUES ($1, 'VOD', '2012-01-01'::date, '2020-06-01'::date),
		       ($1, 'VOD.L', '2020-06-01'::date, NULL)`, isin)
	require.NoError(t, err)
	restore := marketdata.SetQuoteReader(func(_ context.Context, _, _, symbol string) (marketdata.Quote, error) {
		if strings.EqualFold(symbol, "VOD.L") {
			return marketdata.Quote{Exchange: "LSE", Currency: "GBP", Reported: true}, nil
		}
		return marketdata.Quote{Exchange: "NMS", Currency: "USD", Reported: true}, nil
	})
	t.Cleanup(restore)

	meta := supplier.NewMockMarketdata(t)
	meta.On("GetAsset", mock.Anything, "VOD").Return(nil, marketdata.ErrUnknownSymbol)
	meta.On("SearchAsset", mock.Anything, isin).Return([]*pb.Asset{}, nil)
	bars := supplier.NewMockMarketdata(t)
	stream := &countingStream{}
	fs := NewMarketdataServer(pool, meta)
	start := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2020, 12, 31, 0, 0, 0, 0, time.UTC)
	blocked := fs.downloadSymbol(context.Background(), newDownloadSender(stream, func() {}), "VOD", start, &end, "yahoo", "",
		repository.Asset{Conn: pool}, repository.OHLC{Conn: pool}, dividendFree{bars})
	require.Nil(t, blocked)
	require.Len(t, stream.sent, 1)
	require.Equal(t, "Prices for VOD come from VOD.L, the company's later ticker, which is listed outside the US (London, GBP). Global markets aren't supported yet.", stream.sent[0].GetError())
	bars.AssertNotCalled(t, "GetOHLC", mock.Anything, "VOD.L", mock.Anything, mock.Anything)
	var stored int
	require.NoError(t, pool.QueryRow(context.Background(), `SELECT count(*) FROM ohlc`).Scan(&stored))
	require.Zero(t, stored)
}

func TestDownloadHistoricalDataNamesACurrencyOnlyLaterTicker(t *testing.T) {
	test_helper.SetupEnvironment(t, &test_helper.Containers{Postgres: true})
	pool, err := pgxpool.New(context.Background(), environment.GetPostgresURL())
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	require.NoError(t, repository.Recreate(context.Background(), pool))

	const isin = "CA9999999991"
	instruments := repository.Instruments{Conn: pool}
	_, err = instruments.Store(context.Background(), isin, "Example")
	require.NoError(t, err)
	_, err = pool.Exec(context.Background(), `
		INSERT INTO asset_symbol (isin, symbol, from_date, to_date)
		VALUES ($1, 'XYZ', '2012-01-01'::date, '2020-06-01'::date),
		       ($1, 'XYZ2', '2020-06-01'::date, NULL)`, isin)
	require.NoError(t, err)
	restore := marketdata.SetQuoteReader(func(_ context.Context, _, _, symbol string) (marketdata.Quote, error) {
		if strings.EqualFold(symbol, "XYZ2") {
			return marketdata.Quote{Currency: "CAD", Reported: true}, nil
		}
		return marketdata.Quote{Exchange: "NMS", Currency: "USD", Reported: true}, nil
	})
	t.Cleanup(restore)

	meta := supplier.NewMockMarketdata(t)
	meta.On("GetAsset", mock.Anything, "XYZ").Return(nil, marketdata.ErrUnknownSymbol)
	meta.On("SearchAsset", mock.Anything, isin).Return([]*pb.Asset{}, nil)
	bars := supplier.NewMockMarketdata(t)
	stream := &countingStream{}
	fs := NewMarketdataServer(pool, meta)
	start := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2020, 12, 31, 0, 0, 0, 0, time.UTC)
	blocked := fs.downloadSymbol(context.Background(), newDownloadSender(stream, func() {}), "XYZ", start, &end, "yahoo", "",
		repository.Asset{Conn: pool}, repository.OHLC{Conn: pool}, dividendFree{bars})
	require.Nil(t, blocked)
	require.Len(t, stream.sent, 1)
	require.Equal(t, "Prices for XYZ come from XYZ2, the company's later ticker, which yahoo prices in CAD, not USD. Global markets aren't supported yet.", stream.sent[0].GetError())
	bars.AssertNotCalled(t, "GetOHLC", mock.Anything, "XYZ2", mock.Anything, mock.Anything)
}

func TestDownloadSymbolRefusesALondonSuffix(t *testing.T) {
	meta := supplier.NewMockMarketdata(t)
	bars := supplier.NewMockMarketdata(t)
	stream := &countingStream{}
	fs := NewMarketdataServer(nil, meta)
	start := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2020, 12, 31, 0, 0, 0, 0, time.UTC)
	blocked := fs.downloadSymbol(context.Background(), newDownloadSender(stream, func() {}), "VOD.L", start, &end, "yahoo", "",
		repository.Asset{}, repository.OHLC{}, dividendFree{bars})
	require.Nil(t, blocked)
	require.Len(t, stream.sent, 1)
	require.Equal(t, "VOD.L is listed outside the US (London, GBP). Global markets aren't supported yet.", stream.sent[0].GetError())
	meta.AssertNotCalled(t, "GetAsset", mock.Anything, mock.Anything)
	bars.AssertNotCalled(t, "GetOHLC", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}
