package trading_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lhjnilsson/foreverbull/pkg/finance/internal/suppliers/trading"
	"github.com/stretchr/testify/require"
)

func TestEmbeddedListingsSample(t *testing.T) {
	book, err := trading.Load("")
	require.NoError(t, err)

	aapl, ok := book.Get("US0378331005")
	require.True(t, ok)
	require.Equal(t, "AAPL", aapl.Symbol)
	require.Equal(t, "USD", aapl.Currency)
	require.Equal(t, "AAPL", aapl.AlpacaSymbol)
	require.Equal(t, int64(211), aapl.SaxoUIC)
	require.Equal(t, "Stock", aapl.SaxoAssetType)
	require.Equal(t, int64(265598), aapl.IBKRConID)
	require.NotEmpty(t, aapl.IGEpic)
	require.Equal(t, int64(1), aapl.IGShareSize)

	msft, ok := book.Get("us5949181045")
	require.True(t, ok)
	require.Equal(t, "MSFT", msft.Symbol)
	require.Equal(t, "USD", msft.Currency)
	require.Equal(t, "MSFT", msft.AlpacaSymbol)
	require.Equal(t, int64(261), msft.SaxoUIC)
	require.Equal(t, "Stock", msft.SaxoAssetType)
	require.Equal(t, int64(272093), msft.IBKRConID)
	require.NotEmpty(t, msft.IGEpic)
	require.Equal(t, int64(1), msft.IGShareSize)

	var sized int
	for _, row := range book.List() {
		if row.IGShareSize == 10 {
			sized++
			require.NotEmpty(t, row.IGEpic)
			require.NotEmpty(t, row.ISIN)
		}
	}
	require.Equal(t, 1, sized)
}

func TestListingsFileReplacesSample(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "listings.json")
	body := `[
	  {"isin":"US0000000000","symbol":"ZZZ","currency":"EUR","alpaca_symbol":"","saxo_uic":0,"saxo_asset_type":"","ibkr_conid":0,"ig_epic":"","ig_share_size":0}
	]`
	require.NoError(t, os.WriteFile(path, []byte(body), 0o600))

	book, err := trading.Load(path)
	require.NoError(t, err)
	rows := book.List()
	require.Len(t, rows, 1)
	require.Equal(t, "US0000000000", rows[0].ISIN)
	require.Equal(t, "EUR", rows[0].Currency)
	_, ok := book.Get("US0378331005")
	require.False(t, ok)
}

func TestListingsRejectDuplicateISIN(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "listings.json")
	body := `[
	  {"isin":"US0378331005","symbol":"AAPL","currency":"USD"},
	  {"isin":"US0378331005","symbol":"AAPL","currency":"USD"}
	]`
	require.NoError(t, os.WriteFile(path, []byte(body), 0o600))
	book, err := trading.Load(path)
	require.Error(t, err)
	require.Nil(t, book)
}
