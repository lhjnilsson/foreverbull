package trading_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

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
	require.EqualError(t, err, "Couldn't load the trading listings in "+path+": row 2 (US0378331005, no mic) overlaps the dates of an earlier row for the same ISIN with no mic. Fix the file, then restart foreverbull with `uv run fbenv stop && uv run fbenv start`.")
	require.Nil(t, book)
}

func TestListingsStoresOneISINOnTwoVenues(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "listings.json")
	body := `[
	  {"isin":"US0378331005","mic":"XNAS","symbol":"AAPL","currency":"USD","price_unit":"USD"},
	  {"isin":"US0378331005","mic":"XNYS","symbol":"AAPL","currency":"USD","price_unit":"USD"}
	]`
	require.NoError(t, os.WriteFile(path, []byte(body), 0o600))
	book, err := trading.Load(path)
	require.NoError(t, err)
	require.Len(t, book.List(), 2)
	_, ok := book.Get("US0378331005")
	require.False(t, ok, "two venues are not one listing")

	on := time.Date(2024, 1, 2, 15, 0, 0, 0, time.UTC)
	xnas, ok := book.ListingAt("US0378331005", "xnas", on)
	require.True(t, ok)
	require.Equal(t, "XNAS", xnas.MIC)
	require.Equal(t, "USD", xnas.Currency)
	require.Equal(t, "USD", xnas.PriceUnit)
	xnys, ok := book.ListingAt("US0378331005", "XNYS", on)
	require.True(t, ok)
	require.Equal(t, "USD", xnys.Currency)
	require.Equal(t, "USD", xnys.PriceUnit)
}

func TestListingsKeepCurrencyAndPriceUnit(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "listings.json")
	body := `[{"isin":"US0378331005","symbol":"AAPL","currency":"USD"}]`
	require.NoError(t, os.WriteFile(path, []byte(body), 0o600))
	book, err := trading.Load(path)
	require.NoError(t, err)
	row, ok := book.Get("US0378331005")
	require.True(t, ok)
	require.Equal(t, "USD", row.Currency)
	require.Empty(t, row.PriceUnit)
}

func TestListingsFollowTheISINWindow(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "listings.json")
	body := `[
	  {"isin":"US0378331005","mic":"XNAS","symbol":"AAPL","currency":"USD","valid_from":"2020-01-01","valid_to":"2021-01-01"},
	  {"isin":"US0378331005","mic":"XNAS","symbol":"AAPL","currency":"GBP","price_unit":"GBp","valid_from":"2021-01-01"}
	]`
	require.NoError(t, os.WriteFile(path, []byte(body), 0o600))
	book, err := trading.Load(path)
	require.NoError(t, err)
	require.Len(t, book.List(), 2)

	first, ok := book.ListingAt("US0378331005", "XNAS", time.Date(2020, 6, 1, 0, 0, 0, 0, time.UTC))
	require.True(t, ok)
	require.Empty(t, first.PriceUnit)
	_, ok = book.ListingAt("US0378331005", "XNAS", time.Date(2019, 12, 31, 0, 0, 0, 0, time.UTC))
	require.False(t, ok)
	second, ok := book.ListingAt("US0378331005", "XNAS", time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC))
	require.True(t, ok)
	require.Equal(t, "GBX", second.PriceUnit)
	require.Equal(t, "GBP", second.Currency)
}

func TestListingsPriceUnitMapsPenceAndPounds(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "listings.json")
	pence := `[{"isin":"GB0000000001","mic":"XXXX","symbol":"ZZZ","currency":"GBP","price_unit":"GBp"}]`
	require.NoError(t, os.WriteFile(path, []byte(pence), 0o600))
	book, err := trading.Load(path)
	require.NoError(t, err)
	row, ok := book.Get("GB0000000001")
	require.True(t, ok)
	require.Equal(t, "GBP", row.Currency)
	require.Equal(t, "GBX", row.PriceUnit)

	gbp := `[{"isin":"GB0000000002","mic":"XXXX","symbol":"YYY","currency":"gbp","price_unit":"gbp"}]`
	require.NoError(t, os.WriteFile(path, []byte(gbp), 0o600))
	book, err = trading.Load(path)
	require.NoError(t, err)
	row, ok = book.Get("GB0000000002")
	require.True(t, ok)
	require.Equal(t, "GBP", row.Currency)
	require.Equal(t, "GBP", row.PriceUnit)
}

func TestListingsTouchingWindowsAreNotAnOverlapWhenTheLaterRowIsFirst(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "listings.json")
	body := `[
	  {"isin":"US0378331005","mic":"XNAS","symbol":"AAPL","currency":"USD","valid_from":"2021-01-01"},
	  {"isin":"US0378331005","mic":"XNAS","symbol":"AAPL","currency":"USD","valid_from":"2020-01-01","valid_to":"2021-01-01"}
	]`
	require.NoError(t, os.WriteFile(path, []byte(body), 0o600))
	book, err := trading.Load(path)
	require.NoError(t, err)
	require.Len(t, book.List(), 2)

	boundary := time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)
	before, ok := book.ListingAt("US0378331005", "XNAS", time.Date(2020, 6, 1, 0, 0, 0, 0, time.UTC))
	require.True(t, ok)
	require.NotNil(t, before.ValidTo)
	onBoundary, ok := book.ListingAt("US0378331005", "XNAS", boundary)
	require.True(t, ok)
	require.True(t, onBoundary.ValidFrom.Equal(boundary))
	require.Nil(t, onBoundary.ValidTo)
}

func TestListingsFileLoadErrors(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "listings.json")
	load := func(body string) error {
		require.NoError(t, os.WriteFile(path, []byte(body), 0o600))
		book, err := trading.Load(path)
		require.Nil(t, book)
		return err
	}
	sentence := func(problem string) string {
		return "Couldn't load the trading listings in " + path + ": " + problem +
			" Fix the file, then restart foreverbull with `uv run fbenv stop && uv run fbenv start`."
	}

	require.EqualError(t, load(`[{"isin":"US0378331005","symbol":"AAPL","currency":"USD","price_unit":"USX"}]`),
		sentence("row 1 (US0378331005) has price unit 'USX', which isn't a unit of USD."))
	require.EqualError(t, load(`[{"isin":"US0378331005","symbol":"AAPL","currency":"USD","price_unit":"uSx"}]`),
		sentence("row 1 (US0378331005) has price unit 'uSx', which isn't a unit of USD."))
	require.EqualError(t, load(`[{"isin":"US0378331005","symbol":"AAPL","price_unit":"USX"}]`),
		sentence("row 1 (US0378331005) has no currency."))
	require.EqualError(t, load(`[
	  {"isin":"US0378331005","mic":"XNAS","symbol":"AAPL","currency":"USD"},
	  {"isin":"US0378331005","mic":"XNAS","symbol":"AAPL","currency":"USD"}
	]`), sentence("row 2 (US0378331005 on XNAS) overlaps the dates of an earlier row for the same ISIN and exchange."))
	require.EqualError(t, load(`[
	  {"isin":"US0378331005","symbol":"AAPL","currency":"USD"},
	  {"isin":"US0378331005","symbol":"AAPL","currency":"USD"}
	]`), sentence("row 2 (US0378331005, no mic) overlaps the dates of an earlier row for the same ISIN with no mic."))
	require.EqualError(t, load(`[
	  {"isin":"US0378331005","symbol":"AAPL"},
	  {"isin":"US5949181045","symbol":"MSFT","currency":"USD","price_unit":"USX"}
	]`), sentence("row 1 (US0378331005) has no currency; row 2 (US5949181045) has price unit 'USX', which isn't a unit of USD."))
	require.EqualError(t, load(`[{"symbol":"AAPL","currency":"USD"}]`),
		sentence("row 1 has no ISIN."))
	require.EqualError(t, load(`[{"isin":"US0378331005","currency":"USD"}]`),
		sentence("row 1 (US0378331005) has no symbol."))
	require.EqualError(t, load(`[{"isin":"US0378331005","symbol":"AAPL","currency":"USD","valid_from":"2024-13-01"}]`),
		sentence("row 1 (US0378331005) has valid_from '2024-13-01', which isn't a date."))
	require.EqualError(t, load(`[{"isin":"US0378331005","symbol":"AAPL","currency":"USD","valid_to":"yesterday"}]`),
		sentence("row 1 (US0378331005) has valid_to 'yesterday', which isn't a date."))
	require.EqualError(t, load(`[{"isin":"US0378331005","symbol":"AAPL","currency":"USD","valid_from":"2021-01-01","valid_to":"2020-01-01"}]`),
		sentence("row 1 (US0378331005) has valid_to on or before valid_from."))
	require.EqualError(t, load(`[{"isin":"US0378331005","symbol":"AAPL","currency":"USD","ibkr_conid":-1}]`),
		sentence("row 1 (US0378331005) has a negative ibkr_conid."))
	require.EqualError(t, load(`[{"isin":"US0378331005","symbol":"AAPL","currency":"USD","saxo_uic":-1}]`),
		sentence("row 1 (US0378331005) has a negative saxo_uic."))
	require.EqualError(t, load(`[{"isin":"US0378331005","symbol":"AAPL","currency":"USD","ig_share_size":-1}]`),
		sentence("row 1 (US0378331005) has a negative ig_share_size."))
	require.EqualError(t, load(`[{"isin":"US0378331005","symbol":"AAPL","currency":"USD","saxo_uic":211}]`),
		sentence("row 1 (US0378331005) has saxo_uic and no saxo_asset_type."))
	require.EqualError(t, load(`[]`), sentence("the file is empty."))

	bad := json.Unmarshal([]byte("oops"), &[]any{})
	require.EqualError(t, load(`oops`), sentence(fmt.Sprintf("the file is not JSON (%s).", bad)))
}
