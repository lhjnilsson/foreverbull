package marketdata_test

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/internal/test_helper"
	"github.com/lhjnilsson/foreverbull/pkg/finance/internal/repository"
	"github.com/lhjnilsson/foreverbull/pkg/finance/internal/suppliers/marketdata"
	pb "github.com/lhjnilsson/foreverbull/pkg/pb/finance"
	"github.com/stretchr/testify/require"
)

// A Yahoo download through a past end stores the end date's bar: product-e2e
// lost SPY's 2021-03-31 because period2 stopped at 00:00 UTC that day.
func TestYahooDownloadStoresTheEndDatesBar(t *testing.T) {
	test_helper.SetupEnvironment(t, &test_helper.Containers{Postgres: true})
	ctx := context.Background()
	conn, err := pgxpool.New(ctx, environment.GetPostgresURL())
	require.NoError(t, err)
	t.Cleanup(conn.Close)
	require.NoError(t, repository.Recreate(ctx, conn))
	assets := repository.Asset{Conn: conn}
	require.NoError(t, assets.Store(ctx, "SPY", "SPDR S&P 500 ETF Trust"))

	nyc, err := time.LoadLocation("America/New_York")
	require.NoError(t, err)
	now := time.Date(2026, 10, 30, 11, 0, 0, 0, nyc)
	yahoo := marketdata.NewYahooRangeClient(t, now, "2021-03-29", "2021-03-30", "2021-03-31", "2021-04-01")
	start := time.Date(2021, 3, 29, 0, 0, 0, 0, time.UTC)
	end := time.Date(2021, 3, 31, 0, 0, 0, 0, time.UTC)

	ohlc := repository.OHLC{Conn: conn}
	require.NoError(t, ohlc.Sync(ctx, "yahoo", "SPY", start, &end, func(from time.Time, to *time.Time) ([]*pb.OHLC, error) {
		return yahoo.GetOHLC("SPY", from, to)
	}))

	first, last, stored, err := ohlc.Range(ctx, "yahoo", "SPY")
	require.NoError(t, err)
	require.True(t, stored)
	require.Equal(t, "2021-03-29", first.In(nyc).Format(time.DateOnly))
	require.Equal(t, "2021-03-31", last.In(nyc).Format(time.DateOnly))
}
