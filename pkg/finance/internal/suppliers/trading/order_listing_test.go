package trading

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/lhjnilsson/foreverbull/pkg/finance/supplier"
	"github.com/stretchr/testify/require"
)

func TestOrderListingNamesTwoAndThreeExchanges(t *testing.T) {
	day := time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC)
	two := "US0378331005 is listed on more than one exchange in the trading listings (XNAS and XNYS), so foreverbull can't tell where to place the order. Keep one of them in the listings file, restart foreverbull with `uv run fbenv stop && uv run fbenv start`, then deploy the strategy again with the same options."
	three := "US0378331005 is listed on more than one exchange in the trading listings (XNAS, XNYS and BATS), so foreverbull can't tell where to place the order. Keep one of them in the listings file, restart foreverbull with `uv run fbenv stop && uv run fbenv start`, then deploy the strategy again with the same options."

	_, err := orderListing([]Listing{
		{ISIN: "US0378331005", MIC: "XNAS"},
		{ISIN: "US0378331005", MIC: "XNYS"},
	}, "US0378331005", day)
	require.EqualError(t, err, two)
	require.ErrorIs(t, err, supplier.ErrNotTradable)

	_, err = orderListing([]Listing{
		{ISIN: "US0378331005", MIC: "XNAS"},
		{ISIN: "US0378331005", MIC: "XNYS"},
		{ISIN: "US0378331005", MIC: "BATS"},
	}, "US0378331005", day)
	require.EqualError(t, err, three)
	require.ErrorIs(t, err, supplier.ErrNotTradable)
}

func TestOrderListingUsesTheDayTheOrderIsPlaced(t *testing.T) {
	end := time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)
	rows := []Listing{
		{ISIN: "US0378331005", MIC: "XNAS", ValidFrom: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC), ValidTo: &end},
		{ISIN: "US0378331005", MIC: "XNYS", ValidFrom: end},
	}

	got, err := orderListing(rows, "US0378331005", time.Date(2020, 6, 1, 0, 0, 0, 0, time.UTC))
	require.NoError(t, err)
	require.Equal(t, "XNAS", got.MIC)

	got, err = orderListing(rows, "US0378331005", time.Date(2021, 6, 1, 0, 0, 0, 0, time.UTC))
	require.NoError(t, err)
	require.Equal(t, "XNYS", got.MIC)

	_, err = orderListing(rows, "US0378331005", time.Date(2019, 6, 1, 0, 0, 0, 0, time.UTC))
	require.EqualError(t, err, "isin US0378331005: instrument is not tradable")
	require.ErrorIs(t, err, supplier.ErrNotTradable)
}

func TestOrderListingUsesTheBoundaryDayOnOneExchange(t *testing.T) {
	boundary := time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)
	rows := []Listing{
		{ISIN: "US0378331005", MIC: "XNAS", ValidFrom: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC), ValidTo: &boundary},
		{ISIN: "US0378331005", MIC: "XNAS", ValidFrom: boundary},
	}

	got, err := orderListing(rows, "US0378331005", time.Date(2020, 6, 1, 0, 0, 0, 0, time.UTC))
	require.NoError(t, err)
	require.Equal(t, &boundary, got.ValidTo)

	got, err = orderListing(rows, "US0378331005", boundary)
	require.NoError(t, err)
	require.True(t, got.ValidFrom.Equal(boundary))
	require.Nil(t, got.ValidTo)

	got, err = orderListing(rows, "US0378331005", time.Date(2021, 6, 1, 0, 0, 0, 0, time.UTC))
	require.NoError(t, err)
	require.Nil(t, got.ValidTo)
}

func TestOrderListingSkipsAnEmptyMIC(t *testing.T) {
	day := time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC)
	got, err := orderListing([]Listing{
		{ISIN: "US0378331005", MIC: ""},
		{ISIN: "US0378331005", MIC: "XNAS"},
	}, "US0378331005", day)
	require.NoError(t, err)
	require.Equal(t, "XNAS", got.MIC)

	got, err = orderListing([]Listing{
		{ISIN: "US0378331005", MIC: "XNAS"},
		{ISIN: "US0378331005", MIC: ""},
	}, "US0378331005", day)
	require.NoError(t, err)
	require.Equal(t, "XNAS", got.MIC)

	_, err = orderListing([]Listing{
		{ISIN: "US0378331005", MIC: "XNAS"},
		{ISIN: "US0378331005", MIC: ""},
		{ISIN: "US0378331005", MIC: "XNYS"},
	}, "US0378331005", day)
	require.EqualError(t, err, "US0378331005 is listed on more than one exchange in the trading listings (XNAS and XNYS), so foreverbull can't tell where to place the order. Keep one of them in the listings file, restart foreverbull with `uv run fbenv stop && uv run fbenv start`, then deploy the strategy again with the same options.")
	require.NotContains(t, err.Error(), " and )")
	require.NotContains(t, err.Error(), "( and ")
}

func TestOrderListingRefusesSeveralRowsWithNoMIC(t *testing.T) {
	_, err := orderListing([]Listing{
		{ISIN: "US0378331005"},
		{ISIN: "US0378331005"},
	}, "US0378331005", time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC))
	require.EqualError(t, err, "US0378331005 has more than one listing in the trading listings, so foreverbull can't tell where to place the order. Keep one of them in the listings file, restart foreverbull with `uv run fbenv stop && uv run fbenv start`, then deploy the strategy again with the same options.")
	require.ErrorIs(t, err, supplier.ErrNotTradable)
	require.NotContains(t, err.Error(), "XNAS")
}

func TestListingsFileErrorUnwraps(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(dir, "missing.json")
	_, err := Load(missing)
	require.ErrorIs(t, err, os.ErrNotExist)
	var pathErr *fs.PathError
	require.ErrorAs(t, err, &pathErr)

	path := filepath.Join(dir, "bad.json")
	require.NoError(t, os.WriteFile(path, []byte("oops"), 0o600))
	_, err = Load(path)
	require.Error(t, err)
	var syntax *json.SyntaxError
	require.ErrorAs(t, err, &syntax)
}
