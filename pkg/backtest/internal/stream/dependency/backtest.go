package dependency

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/lhjnilsson/foreverbull/internal/stream"
)

const (
	GetETFScraper       stream.Dependency = "get_etf_scraper"
	GetMarketdataClient stream.Dependency = "get_marketdata_client"

	// snapshotLookbackDays is how far GetSnapshot walks back when asOf has empty aaData.
	snapshotLookbackDays = 6
)

// FOREVERBULL_ETF_SNAPSHOT_CACHE overrides the runtime JSON cache directory.
const etfSnapshotCacheEnv = "FOREVERBULL_ETF_SNAPSHOT_CACHE"

type ETFScraper interface {
	GetAssets(name string, years ...string) (map[string][]Asset, error)
	GetSnapshot(etf string, asOf time.Time) ([]Asset, error)
	ListSnapshotDates(etf string, start, end time.Time) ([]time.Time, error)
	GetBenchmark(name string) (string, error)
}

type Response struct {
	Data [][]interface{} `json:"aaData"`
}

type Asset struct {
	Symbol    string
	Name      string
	Sector    string
	AssetType string
	ISIN      string
}

func NewETFScaper() ETFScraper {
	return &ISharesETFLoopup{
		products: defaultETFProducts(),
		cache:    &fsSnapshotCache{dir: defaultSnapshotCacheDir()},
		client:   http.DefaultClient,
	}
}

func defaultETFProducts() map[string]string {
	products := ETFCatalog()
	urls := make(map[string]string, len(products))
	for _, product := range products {
		urls[product.Name] = product.URL
	}
	return urls
}

func defaultSnapshotCacheDir() string {
	if dir := os.Getenv(etfSnapshotCacheEnv); dir != "" {
		return dir
	}
	base, err := os.UserCacheDir()
	if err != nil || base == "" {
		return filepath.Join(os.TempDir(), "foreverbull", "etf_snapshots")
	}
	return filepath.Join(base, "foreverbull", "etf_snapshots")
}

type ISharesETFLoopup struct {
	products map[string]string
	cache    snapshotCache
	client   *http.Client
}

func (e *ISharesETFLoopup) productURL(etf string) (string, error) {
	baseurl, found := e.products[etf]
	if !found {
		return "", fmt.Errorf("ETF not found: %s", etf)
	}
	return baseurl, nil
}

func (e *ISharesETFLoopup) httpClient() *http.Client {
	if e.client != nil {
		return e.client
	}
	return http.DefaultClient
}

// GetAssets is a year-keyed compatibility wrapper over month-end snapshots.
// Each year is the ISIN union of ListSnapshotDates in that calendar year (later ticker wins).
func (e *ISharesETFLoopup) GetAssets(etf string, years ...string) (map[string][]Asset, error) {
	if _, err := e.productURL(etf); err != nil {
		return nil, err
	}
	retMap := make(map[string][]Asset, len(years))
	for _, year := range years {
		start, err := time.Parse("2006-01-02", year+"-01-01")
		if err != nil {
			return nil, fmt.Errorf("error parsing date: %w", err)
		}
		end := start.AddDate(1, 0, -1)
		dates, err := e.ListSnapshotDates(etf, start, end)
		if err != nil {
			return nil, err
		}
		byISIN := make(map[string]Asset)
		for _, asOf := range dates {
			assets, err := e.GetSnapshot(etf, asOf)
			if err != nil {
				return nil, err
			}
			for _, asset := range assets {
				byISIN[asset.ISIN] = asset
			}
		}
		retMap[year] = sortedAssets(byISIN)
	}
	return retMap, nil
}

func (e *ISharesETFLoopup) GetBenchmark(etf string) (string, error) {
	for _, product := range etfCatalog {
		if product.Name == etf {
			return product.Benchmark, nil
		}
	}
	return "", fmt.Errorf("ETF not found: %s", etf)
}

func sortedAssets(byISIN map[string]Asset) []Asset {
	assets := make([]Asset, 0, len(byISIN))
	for _, asset := range byISIN {
		assets = append(assets, asset)
	}
	sort.Slice(assets, func(i, j int) bool {
		if assets[i].Symbol == assets[j].Symbol {
			return assets[i].ISIN < assets[j].ISIN
		}
		return assets[i].Symbol < assets[j].Symbol
	})
	return assets
}
