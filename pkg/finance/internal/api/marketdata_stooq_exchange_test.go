package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/internal/test_helper"
	"github.com/lhjnilsson/foreverbull/pkg/finance/internal/repository"
	"github.com/lhjnilsson/foreverbull/pkg/finance/internal/suppliers/marketdata"
	"github.com/lhjnilsson/foreverbull/pkg/finance/supplier"
	pb "github.com/lhjnilsson/foreverbull/pkg/pb/finance"
	servicePb "github.com/lhjnilsson/foreverbull/pkg/pb/finance/service"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// shopTORefusal is the download's refusal for a non-US suffix. The quote is
// not asked, and neither is a later ticker. The global-markets sentence follows.
const shopTORefusal = "SHOP.TO is listed outside the US (the .TO suffix isn't a US exchange). Global markets aren't supported yet."

// shopISIN is the instrument a later-ticker lookup would resolve SHOP.TO to.
const shopISIN = "CA82509L1076"

// countingPrice counts GetOHLC calls. The inner client does the fetch.
type countingPrice struct {
	supplier.Marketdata
	calls   int
	symbols []string
}

func (c *countingPrice) GetOHLC(ctx context.Context, symbol string, start time.Time, end *time.Time) ([]*pb.OHLC, error) {
	c.calls++
	c.symbols = append(c.symbols, symbol)
	return c.Marketdata.GetOHLC(ctx, symbol, start, end)
}

// SHOP.TO is listed outside the US. The download stops before GetAsset,
// before GetOHLC, and before the later ticker a missing price would look up.
func TestDownloadSymbolRefusesUnsupportedStooqExchangeOnce(t *testing.T) {
	test_helper.SetupEnvironment(t, &test_helper.Containers{Postgres: true})
	pool, err := pgxpool.New(context.Background(), environment.GetPostgresURL())
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	require.NoError(t, repository.Recreate(context.Background(), pool))

	ctx := context.Background()
	instruments := repository.Instruments{Conn: pool}
	_, err = instruments.Store(ctx, shopISIN, "Shopify Inc")
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `
		INSERT INTO asset_symbol (isin, symbol, from_date, to_date)
		VALUES ($1, 'SHOP.TO', '2015-01-01'::date, NULL)`, shopISIN)
	require.NoError(t, err)

	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits++
		http.Error(w, "no data", http.StatusNotFound)
	}))
	t.Cleanup(srv.Close)
	client, err := marketdata.NewStooqClient(
		marketdata.WithStooqExchange("US"),
		marketdata.WithStooqBaseURL(srv.URL),
	)
	require.NoError(t, err)
	bars := &countingPrice{Marketdata: client}

	meta := supplier.NewMockMarketdata(t)

	stream := &countingStream{}
	start := time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC)
	end := time.Date(2024, 3, 28, 0, 0, 0, 0, time.UTC)
	blocked := NewMarketdataServer(pool, meta).downloadSymbol(ctx, newDownloadSender(stream, func() {}),
		"SHOP.TO", start, &end, "stooq", "", repository.Asset{Conn: pool}, repository.OHLC{Conn: pool}, bars)

	require.Nil(t, blocked)
	require.Len(t, stream.sent, 1)
	require.Equal(t, "SHOP.TO", stream.sent[0].GetSymbol())
	require.Equal(t, servicePb.DownloadHistoricalDataResponse_STATUS_ERROR, stream.sent[0].GetStatus())
	require.Equal(t, shopTORefusal, stream.sent[0].GetError())
	require.Nil(t, stream.sent[0].GetLaterTicker())
	require.Zero(t, bars.calls)
	require.Empty(t, bars.symbols)
	require.Zero(t, hits)
	meta.AssertNotCalled(t, "SearchAsset", mock.Anything, mock.Anything)
	meta.AssertNotCalled(t, "GetAsset", mock.Anything, mock.Anything)

	var stored, later int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM ohlc`).Scan(&stored))
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM asset_symbol WHERE symbol = 'SHOP'`).Scan(&later))
	require.Zero(t, stored)
	require.Zero(t, later)
}
