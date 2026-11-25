package api

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/internal/test_helper"
	"github.com/lhjnilsson/foreverbull/pkg/finance/internal/mapping"
	"github.com/lhjnilsson/foreverbull/pkg/finance/internal/repository"
	"github.com/lhjnilsson/foreverbull/pkg/finance/supplier"
	pb "github.com/lhjnilsson/foreverbull/pkg/pb/finance"
	"github.com/stretchr/testify/require"
)

// A fresh BRK.B download has no listings file and no asset_symbol row.
// The separated spelling is learned before it is stored as BRKB, so Yahoo
// is asked for BRK-B.
func TestDownloadSymbolsLearnsSeparatedClassBeforeFolding(t *testing.T) {
	got, err := downloadSymbols([]string{"BRK.B"})
	require.NoError(t, err)
	require.Equal(t, []string{"BRKB"}, got)
	require.Equal(t, "BRK-B", mapping.FormatYahoo(mapping.Process(), "BRKB"))
	require.Equal(t, "BRK.B", mapping.FormatAlpaca(mapping.Process(), "BRKB"))
}

type joinedClassSearcher struct {
	supplier.Marketdata
	classes *mapping.Classes
	hit     string
	isin    string
}

func (s joinedClassSearcher) ShareClasses() *mapping.Classes { return s.classes }

func (s joinedClassSearcher) SearchAsset(context.Context, string) ([]*pb.Asset, error) {
	return []*pb.Asset{{Symbol: s.hit, ISIN: s.isin}}, nil
}

// The iShares ticker is stored with its ISIN before download. No listings
// file. The provider's symbol for that ISIN teaches BRKB, including a hit
// with an empty ISIN which must not.
func TestLearnJoinedShareClassFromStoredISIN(t *testing.T) {
	test_helper.SetupEnvironment(t, &test_helper.Containers{Postgres: true})
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, environment.GetPostgresURL())
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	require.NoError(t, repository.Recreate(ctx, pool))

	const (
		isin   = "US0846707026"
		symbol = "BRKB"
	)
	instruments := repository.Instruments{Conn: pool}
	_, err = instruments.Store(ctx, isin, "Berkshire Hathaway Inc.")
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `
		INSERT INTO asset_symbol (isin, symbol, from_date)
		VALUES ($1, $2, '2020-01-01')`, isin, symbol)
	require.NoError(t, err)

	classes := mapping.NewClasses()
	fs := &MarketdataServer{
		pgx: pool,
		marketdata: joinedClassSearcher{
			classes: classes,
			hit:     "BRK-B",
			isin:    isin,
		},
	}
	learnJoinedShareClass(ctx, fs, symbol, time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC))
	require.Equal(t, "BRK-B", mapping.FormatYahoo(classes, symbol))

	empty := mapping.NewClasses()
	fs.marketdata = joinedClassSearcher{classes: empty, hit: "BRK-B", isin: ""}
	learnJoinedShareClass(ctx, fs, symbol, time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC))
	require.Equal(t, "BRKB", mapping.FormatYahoo(empty, symbol))
}
