package dependency

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/universe"
	"github.com/stretchr/testify/require"
)

func testDate(t *testing.T, s string) time.Time {
	t.Helper()
	d, err := time.Parse("2006-01-02", s)
	require.NoError(t, err)
	return d
}

func holdingsJSON(t *testing.T, rows [][]interface{}) []byte {
	t.Helper()
	body, err := json.Marshal(Response{Data: rows})
	require.NoError(t, err)
	return body
}

func TestListSnapshotDatesUnknownETF(t *testing.T) {
	t.Parallel()

	_, err := NewETFScaper().ListSnapshotDates("unknown", testDate(t, "2022-01-01"), testDate(t, "2022-12-31"))
	require.ErrorContains(t, err, "ETF not found")
}

func TestListSnapshotDatesMonthEndsAndCached(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		calls.Add(1)
	}))
	t.Cleanup(srv.Close)

	cacheDir := t.TempDir()
	cache := &fsSnapshotCache{dir: cacheDir}
	require.NoError(t, cache.store("iShares S&P 100", testDate(t, "2022-06-15"), []byte(`{"aaData":[]}`)))

	scraper := &ISharesETFLoopup{
		products: map[string]string{"iShares S&P 100": srv.URL},
		cache:    cache,
		client:   srv.Client(),
	}

	dates, err := scraper.ListSnapshotDates("iShares S&P 100", testDate(t, "2022-01-01"), testDate(t, "2022-12-31"))
	require.NoError(t, err)
	require.Equal(t, int32(0), calls.Load())
	require.Len(t, dates, 13)
	require.Equal(t, "20220131", dates[0].Format("20060102"))
	require.Equal(t, "20220615", dates[5].Format("20060102"))
	require.Equal(t, "20220630", dates[6].Format("20060102"))
	require.Equal(t, "20221231", dates[12].Format("20060102"))
}

func TestListSnapshotDatesPartialYear(t *testing.T) {
	t.Parallel()

	scraper := &ISharesETFLoopup{
		products: defaultETFProducts(),
	}
	dates, err := scraper.ListSnapshotDates("iShares S&P 100", testDate(t, "2022-01-15"), testDate(t, "2022-03-15"))
	require.NoError(t, err)
	require.Equal(t, []string{"20220131", "20220228"}, []string{
		dates[0].Format("20060102"),
		dates[1].Format("20060102"),
	})
	require.Len(t, dates, 2)
}

func TestGetSnapshotUnknownETF(t *testing.T) {
	t.Parallel()

	_, err := NewETFScaper().GetSnapshot("unknown", testDate(t, "2022-12-31"))
	require.ErrorContains(t, err, "ETF not found")
}

func TestGetSnapshotEquityOnlyAndCache(t *testing.T) {
	t.Parallel()

	body := holdingsJSON(t, [][]interface{}{
		{"AAPL", "Apple Inc.", "Technology", "Equity", "", "", "", "", "US0378331005"},
		{"CASH_USD", "USD Cash", "-", "Cash", "", "", "", "", ""},
		{"BOND", "Some Bond", "Fixed Income", "Fixed Income", "", "", "", "", "US0000000001"},
	})

	var calls atomic.Int32
	var lastAsOf atomic.Value
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		lastAsOf.Store(r.URL.Query().Get("asOfDate"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)

	scraper := &ISharesETFLoopup{
		products: map[string]string{"iShares S&P 100": srv.URL},
		cache:    &fsSnapshotCache{dir: t.TempDir()},
		client:   srv.Client(),
	}

	asOf := testDate(t, "2022-12-31")
	assets, err := scraper.GetSnapshot("iShares S&P 100", asOf)
	require.NoError(t, err)
	require.Equal(t, "20221231", lastAsOf.Load())
	require.Equal(t, int32(1), calls.Load())
	require.Equal(t, []Asset{{
		Symbol:    "AAPL",
		Name:      "Apple Inc.",
		Sector:    "Technology",
		AssetType: "Equity",
		ISIN:      "US0378331005",
	}}, assets)

	assets, err = scraper.GetSnapshot("iShares S&P 100", asOf)
	require.NoError(t, err)
	require.Equal(t, int32(1), calls.Load())
	require.Equal(t, "AAPL", assets[0].Symbol)
	require.Equal(t, "US0378331005", assets[0].ISIN)
}

func TestGetSnapshotTestdataCacheDoesNotHTTP(t *testing.T) {
	t.Parallel()

	scraper := &ISharesETFLoopup{
		products: map[string]string{"iShares S&P 100": "http://127.0.0.1:1"},
		cache:    &fsSnapshotCache{dir: filepath.Join("testdata", "etf_snapshots")},
		client:   &http.Client{Timeout: time.Nanosecond},
	}

	dec, err := scraper.GetSnapshot("iShares S&P 100", testDate(t, "2022-12-31"))
	require.NoError(t, err)
	require.Equal(t, []Asset{
		{
			Symbol:    "AAPL",
			Name:      "Apple Inc.",
			Sector:    "Technology",
			AssetType: "Equity",
			ISIN:      "US0378331005",
		},
		{
			Symbol:    "FB",
			Name:      "Meta Platforms Inc.",
			Sector:    "Communication",
			AssetType: "Equity",
			ISIN:      "US30303M1027",
		},
	}, dec)

	jan, err := scraper.GetSnapshot("iShares S&P 100", testDate(t, "2023-01-31"))
	require.NoError(t, err)
	require.Equal(t, []Asset{
		{
			Symbol:    "AAPL",
			Name:      "Apple Inc.",
			Sector:    "Technology",
			AssetType: "Equity",
			ISIN:      "US0378331005",
		},
		{
			Symbol:    "META",
			Name:      "Meta Platforms Inc.",
			Sector:    "Communication",
			AssetType: "Equity",
			ISIN:      "US30303M1027",
		},
		{
			Symbol:    "MSFT",
			Name:      "Microsoft Corp.",
			Sector:    "Technology",
			AssetType: "Equity",
			ISIN:      "US5949181045",
		},
	}, jan)
}

func TestGetSnapshotLookbackAndUnpublished(t *testing.T) {
	t.Parallel()

	published := holdingsJSON(t, [][]interface{}{
		{"AAPL", "Apple Inc.", "Technology", "Equity", "", "", "", "", "US0378331005"},
	})
	empty := holdingsJSON(t, nil)

	var calls atomic.Int32
	var mu sync.Mutex
	asOfs := make([]string, 0, 8)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		asOf := r.URL.Query().Get("asOfDate")
		mu.Lock()
		asOfs = append(asOfs, asOf)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if asOf == "20221230" {
			_, _ = w.Write(published)
			return
		}
		_, _ = w.Write(empty)
	}))
	t.Cleanup(srv.Close)

	scraper := &ISharesETFLoopup{
		products: map[string]string{"iShares S&P 100": srv.URL},
		cache:    &fsSnapshotCache{dir: t.TempDir()},
		client:   srv.Client(),
	}

	assets, err := scraper.GetSnapshot("iShares S&P 100", testDate(t, "2022-12-31"))
	require.NoError(t, err)
	mu.Lock()
	require.Equal(t, []string{"20221231", "20221230"}, asOfs)
	mu.Unlock()
	require.Equal(t, int32(2), calls.Load())
	require.Equal(t, "AAPL", assets[0].Symbol)

	assets, err = scraper.GetSnapshot("iShares S&P 100", testDate(t, "2022-12-31"))
	require.NoError(t, err)
	require.Equal(t, int32(2), calls.Load())
	require.Equal(t, "US0378331005", assets[0].ISIN)

	emptyScraper := &ISharesETFLoopup{
		products: map[string]string{"iShares S&P 100": srv.URL},
		cache:    &fsSnapshotCache{dir: t.TempDir()},
		client:   srv.Client(),
	}
	calls.Store(0)
	assets, err = emptyScraper.GetSnapshot("iShares S&P 100", testDate(t, "2021-06-30"))
	require.NoError(t, err)
	require.Empty(t, assets)
	require.Equal(t, int32(snapshotLookbackDays+1), calls.Load())
	require.Less(t, int(calls.Load()), 100)
}

func TestYearOfSnapshotsIsNotDailyScan(t *testing.T) {
	t.Parallel()

	body := holdingsJSON(t, [][]interface{}{
		{"AAPL", "Apple Inc.", "Technology", "Equity", "", "", "", "", "US0378331005"},
	})

	var calls atomic.Int32
	var mu sync.Mutex
	asOfs := make([]string, 0, 12)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		mu.Lock()
		asOfs = append(asOfs, r.URL.Query().Get("asOfDate"))
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)

	scraper := &ISharesETFLoopup{
		products: map[string]string{"iShares S&P 100": srv.URL},
		cache:    &fsSnapshotCache{dir: t.TempDir()},
		client:   srv.Client(),
	}

	start := testDate(t, "2022-01-01")
	end := testDate(t, "2022-12-31")
	dates, err := scraper.ListSnapshotDates("iShares S&P 100", start, end)
	require.NoError(t, err)
	require.Len(t, dates, 12)

	for _, asOf := range dates {
		assets, err := scraper.GetSnapshot("iShares S&P 100", asOf)
		require.NoError(t, err)
		require.Equal(t, "US0378331005", assets[0].ISIN)
	}

	require.Equal(t, 12, int(calls.Load()))
	require.Less(t, int(calls.Load()), 100)
	mu.Lock()
	require.Equal(t, []string{
		"20220131", "20220228", "20220331", "20220430", "20220531", "20220630",
		"20220731", "20220831", "20220930", "20221031", "20221130", "20221231",
	}, asOfs)
	mu.Unlock()
}

func TestEtfCacheSlug(t *testing.T) {
	t.Parallel()
	require.Equal(t, "iShares_S_P_100", etfCacheSlug("iShares S&P 100"))
}

func TestGetSnapshotHTMLIsAGap(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<!DOCTYPE html><html></html>"))
	}))
	t.Cleanup(srv.Close)

	scraper := &ISharesETFLoopup{
		products: map[string]string{"iShares S&P 100": srv.URL + "/ishares-sp-100-etf/1467271812596.ajax"},
		cache:    &fsSnapshotCache{dir: t.TempDir()},
		client:   srv.Client(),
	}

	assets, err := scraper.GetSnapshot("iShares S&P 100", testDate(t, "2020-01-31"))
	require.NoError(t, err)
	require.Empty(t, assets)
	require.Equal(t, int32(1), calls.Load())
}

func TestLoadSnapshotWindowFillsLeadingHTMLFromLaterListing(t *testing.T) {
	t.Parallel()

	const isin = "US0378331005"
	body := holdingsJSON(t, [][]interface{}{
		{"AAPL", "Apple Inc.", "Technology", "Equity", "", "", "", "", isin},
		{"CASH_USD", "USD Cash", "-", "Cash", "", "", "", "", ""},
	})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("asOfDate") == "20200229" {
			_, _ = w.Write(body)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<!DOCTYPE html><html></html>"))
	}))
	t.Cleanup(srv.Close)

	scraper := &ISharesETFLoopup{
		products: map[string]string{"iShares S&P 100": srv.URL},
		cache:    &fsSnapshotCache{dir: t.TempDir()},
		client:   srv.Client(),
	}
	start := testDate(t, "2020-01-01")
	end := testDate(t, "2020-02-29")
	listings, err := LoadSnapshotWindow(scraper, "iShares S&P 100", start, end)
	require.NoError(t, err)
	require.Len(t, listings, 2)
	require.Equal(t, "2020-01-31", listings[0].AsOf.Format("2006-01-02"))
	require.True(t, listings[0].Leading)
	require.Equal(t, "2020-02-29", listings[0].FilledFrom.Format("2006-01-02"))
	require.Equal(t, isin, listings[0].Assets[0].ISIN)
	require.Equal(t, "AAPL", listings[0].Assets[0].Symbol)
	require.Equal(t, listings[1].Assets, listings[0].Assets)
	require.False(t, listings[1].Leading)
	require.True(t, listings[1].FilledFrom.IsZero())

	snaps := listingsToSnapshots(listings)
	result := universe.Diff(snaps)
	require.Len(t, result.Members, 1)
	require.Equal(t, isin, result.Members[0].ISIN)
	require.Equal(t, "2020-01-31", result.Members[0].FromDate.Format("2006-01-02"))
	require.Nil(t, result.Members[0].ToDate)
	require.Equal(t, []universe.Alias{{
		ISIN:     isin,
		Symbol:   "AAPL",
		FromDate: testDate(t, "2020-01-31"),
		ToDate:   nil,
	}}, result.Aliases)
}

func TestLoadSnapshotWindowNoHoldingsNamesWindow(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<html></html>"))
	}))
	t.Cleanup(srv.Close)

	scraper := &ISharesETFLoopup{
		products: map[string]string{"iShares S&P 100": srv.URL},
		cache:    &fsSnapshotCache{dir: t.TempDir()},
		client:   srv.Client(),
	}
	_, err := LoadSnapshotWindow(scraper, "iShares S&P 100", testDate(t, "2020-01-01"), testDate(t, "2020-03-31"))
	require.EqualError(t, err, "iShares has no holdings file for iShares S&P 100 between 2020-01-01 and 2020-03-31")
	require.NotContains(t, err.Error(), "invalid character")
}

func TestFillListingsCarryForwardIsNotADropOrRename(t *testing.T) {
	t.Parallel()

	const metaISIN = "US30303M1027"
	fb := []Asset{{Symbol: "FB", Name: "Meta Platforms Inc.", Sector: "Communication", AssetType: "Equity", ISIN: metaISIN}}
	meta := []Asset{{Symbol: "META", Name: "Meta Platforms Inc.", Sector: "Communication", AssetType: "Equity", ISIN: metaISIN}}
	listings := []Listing{
		{AsOf: testDate(t, "2020-01-31"), Assets: fb},
		{AsOf: testDate(t, "2020-02-29")},
		{AsOf: testDate(t, "2020-03-31"), Assets: meta},
	}
	err := fillListings("iShares S&P 100", testDate(t, "2020-01-01"), testDate(t, "2020-03-31"), listings)
	require.NoError(t, err)
	require.Equal(t, "FB", listings[1].Assets[0].Symbol)
	require.Equal(t, metaISIN, listings[1].Assets[0].ISIN)
	require.False(t, listings[1].Leading)
	require.Equal(t, "2020-01-31", listings[1].FilledFrom.Format("2006-01-02"))
	require.Equal(t, "FB", fb[0].Symbol)

	result := universe.Diff(listingsToSnapshots(listings))
	require.Equal(t, []universe.Member{{
		ISIN:     metaISIN,
		FromDate: testDate(t, "2020-01-31"),
		ToDate:   nil,
	}}, result.Members)
	mar := testDate(t, "2020-03-31")
	require.Equal(t, []universe.Alias{
		{ISIN: metaISIN, Symbol: "FB", FromDate: testDate(t, "2020-01-31"), ToDate: &mar},
		{ISIN: metaISIN, Symbol: "META", FromDate: mar, ToDate: nil},
	}, result.Aliases)
}

func listingsToSnapshots(listings []Listing) []universe.Snapshot {
	snaps := make([]universe.Snapshot, 0, len(listings))
	for _, listing := range listings {
		holdings := make([]universe.Holding, 0, len(listing.Assets))
		for _, asset := range listing.Assets {
			holdings = append(holdings, universe.Holding{
				ISIN:      asset.ISIN,
				Symbol:    asset.Symbol,
				Name:      asset.Name,
				AssetType: asset.AssetType,
			})
		}
		snaps = append(snaps, universe.Snapshot{AsOf: listing.AsOf, Holdings: holdings})
	}
	return snaps
}
