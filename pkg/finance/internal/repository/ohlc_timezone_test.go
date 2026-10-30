package repository_test

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/pkg/finance/internal/repository"
	pb "github.com/lhjnilsson/foreverbull/pkg/pb/finance"
)

// newYorkPool is a pool whose every session runs with TimeZone
// America/New_York, so date-keyed SQL that depended on the session zone would
// put 2020-08-26T02:00Z on 2020-08-25.
func (test *OHLCTests) newYorkPool() *pgxpool.Pool {
	cfg, err := pgxpool.ParseConfig(environment.GetPostgresURL())
	test.Require().NoError(err)
	cfg.ConnConfig.RuntimeParams["timezone"] = "America/New_York"
	cfg.MaxConns = 1
	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	test.Require().NoError(err)
	test.T().Cleanup(pool.Close)

	var zone string
	test.Require().NoError(pool.QueryRow(context.Background(), `SHOW TimeZone`).Scan(&zone))
	test.Require().Equal("America/New_York", zone, "the setting must apply to the session running the queries")
	return pool
}

// Trading-date keys don't depend on the session time zone. With TimeZone
// America/New_York, a 00:00Z row replaced by a 05:00Z bar for the same day
// (and a 02:00Z bar, which is still the 25th in New York) leaves exactly one
// row per day at the new scale, through ReplaceRange, StoreBars and Close.
func (test *OHLCTests) TestTradingDateKeysIgnoreSessionTimeZone() {
	ctx := context.Background()
	ny := repository.OHLC{Conn: test.newYorkPool()}
	midnight := time.Date(2020, 8, 26, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 2; i++ {
		test.Require().NoError(ny.Store(ctx, "yahoo", "ABC", midnight.AddDate(0, 0, i), 500, 500, 500, 500, 100))
	}

	morning := midnight.Add(5 * time.Hour)
	test.Require().NoError(ny.ReplaceRange(ctx, "yahoo", "ABC", morning, &morning,
		[]*pb.OHLC{closeBar(morning, 125)}))
	test.Equal(2, test.countRows())
	test.Equal(map[string]float64{"2020-08-26": 125, "2020-08-27": 500}, test.storedCloses())

	early := midnight.AddDate(0, 0, 1).Add(2 * time.Hour)
	test.Require().NoError(ny.StoreBars(ctx, "yahoo", "ABC", []*pb.OHLC{closeBar(early, 126)}))
	test.Equal(2, test.countRows())
	test.Equal(map[string]float64{"2020-08-26": 125, "2020-08-27": 126}, test.storedCloses())

	closePx, err := ny.Close(ctx, "yahoo", "ABC", midnight)
	test.Require().NoError(err)
	test.InDelta(125, closePx, 0.0001)
	closePx, err = ny.Close(ctx, "yahoo", "ABC", early)
	test.Require().NoError(err)
	test.InDelta(126, closePx, 0.0001)
}
