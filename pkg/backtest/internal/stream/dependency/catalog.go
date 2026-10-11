package dependency

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"time"
)

//go:embed etf_catalog.json
var etfCatalogJSON []byte

// ETFProduct is one iShares fund the CLI can select and the scraper can download.
type ETFProduct struct {
	Name      string
	URL       string
	Benchmark string
	Inception time.Time
}

type catalogEntry struct {
	Name      string `json:"name"`
	URL       string `json:"url"`
	Benchmark string `json:"benchmark"`
	Inception string `json:"inception"`
}

var etfCatalog = mustLoadETFCatalog(etfCatalogJSON)

func mustLoadETFCatalog(raw []byte) []ETFProduct {
	products, err := loadETFCatalog(raw)
	if err != nil {
		panic(err)
	}
	return products
}

func loadETFCatalog(raw []byte) ([]ETFProduct, error) {
	var entries []catalogEntry
	if err := json.Unmarshal(raw, &entries); err != nil {
		return nil, fmt.Errorf("ETF catalog: %w", err)
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("ETF catalog is empty")
	}
	products := make([]ETFProduct, 0, len(entries))
	seen := make(map[string]struct{}, len(entries))
	for _, entry := range entries {
		if entry.Name == "" || entry.URL == "" || entry.Benchmark == "" || entry.Inception == "" {
			return nil, fmt.Errorf("ETF catalog entry %q is missing name, url, benchmark, or inception", entry.Name)
		}
		if _, ok := seen[entry.Name]; ok {
			return nil, fmt.Errorf("ETF catalog duplicate name %q", entry.Name)
		}
		seen[entry.Name] = struct{}{}
		inception, err := time.Parse("2006-01-02", entry.Inception)
		if err != nil {
			return nil, fmt.Errorf("ETF catalog inception for %q: %w", entry.Name, err)
		}
		products = append(products, ETFProduct{
			Name:      entry.Name,
			URL:       entry.URL,
			Benchmark: entry.Benchmark,
			Inception: dateOnly(inception),
		})
	}
	return products, nil
}

// ETFCatalog returns the known iShares funds (name, product URL, benchmark, inception).
// The CLI reads the same etf_catalog.json file this package embeds.
func ETFCatalog() []ETFProduct {
	products := make([]ETFProduct, len(etfCatalog))
	copy(products, etfCatalog)
	return products
}
