package trading

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// Sample broker ids are what an adapter sends. The file is a test fixture, not a
// market catalog. Published ids:
//
//   - Saxo AAPL UIC 211, AssetType Stock:
//     https://www.developer.saxo/openapi/learn/extended-assettypes
//     ISIN US0378331005 with Uic 211:
//     https://www.developer.saxo/openapi/learn/securities-transfers
//   - Saxo MSFT UIC 261, ISIN US5949181045:
//     https://www.developer.saxo/fix/message-definitions/symbology-example
//   - IBKR AAPL conid 265598:
//     https://interactivebrokers.github.io/tws-api/market_scanners.html
//   - IBKR MSFT conid 272093 for ISIN US5949181045.
//
// IG epics are the sample deal ids (Apple's minimum size is one share:
// https://www.ig.com/uk/shares/markets-shares/apple-inc-AAPL-US).
// US1912161007 is the row whose ig_share_size is 10.
//
//go:embed listings_sample.json
var sampleListingsJSON []byte

// Listing maps one ISIN to broker ids. Empty or zero broker ids mean that
// broker cannot trade the row.
type Listing struct {
	ISIN          string
	Symbol        string
	Currency      string
	AlpacaSymbol  string
	SaxoUIC       int64
	SaxoAssetType string
	IBKRConID     int64
	IGEpic        string
	IGShareSize   int64
}

// Book is the listing set adapters read. It does not resolve ticker aliases.
type Book struct {
	rows   []Listing
	byISIN map[string]Listing
}

type listingJSON struct {
	ISIN          string `json:"isin"`
	Symbol        string `json:"symbol"`
	Currency      string `json:"currency"`
	AlpacaSymbol  string `json:"alpaca_symbol"`
	SaxoUIC       int64  `json:"saxo_uic"`
	SaxoAssetType string `json:"saxo_asset_type"`
	IBKRConID     int64  `json:"ibkr_conid"`
	IGEpic        string `json:"ig_epic"`
	IGShareSize   int64  `json:"ig_share_size"`
}

// Load returns the embedded sample when path is empty. A non-empty path
// replaces that sample. Callers pass TRADING_LISTINGS_FILE; Load does not
// read the environment.
func Load(path string) (*Book, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return parseListings(sampleListingsJSON)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("listings file: %w", err)
	}
	book, err := parseListings(raw)
	if err != nil {
		return nil, fmt.Errorf("listings file: %w", err)
	}
	return book, nil
}

func parseListings(raw []byte) (*Book, error) {
	var decoded []listingJSON
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return nil, fmt.Errorf("listings: %w", err)
	}
	if len(decoded) == 0 {
		return nil, fmt.Errorf("listings: empty")
	}

	book := &Book{
		rows:   make([]Listing, 0, len(decoded)),
		byISIN: make(map[string]Listing, len(decoded)),
	}
	for i, row := range decoded {
		listing, err := normalizeListing(row)
		if err != nil {
			return nil, fmt.Errorf("listings row %d: %w", i, err)
		}
		if _, exists := book.byISIN[listing.ISIN]; exists {
			return nil, fmt.Errorf("listings row %d: duplicate isin %s", i, listing.ISIN)
		}
		book.byISIN[listing.ISIN] = listing
		book.rows = append(book.rows, listing)
	}
	return book, nil
}

func normalizeListing(row listingJSON) (Listing, error) {
	listing := Listing{
		ISIN:          normalizeISIN(row.ISIN),
		Symbol:        strings.TrimSpace(row.Symbol),
		Currency:      strings.ToUpper(strings.TrimSpace(row.Currency)),
		AlpacaSymbol:  strings.TrimSpace(row.AlpacaSymbol),
		SaxoUIC:       row.SaxoUIC,
		SaxoAssetType: strings.TrimSpace(row.SaxoAssetType),
		IBKRConID:     row.IBKRConID,
		IGEpic:        strings.TrimSpace(row.IGEpic),
		IGShareSize:   row.IGShareSize,
	}
	if listing.ISIN == "" {
		return Listing{}, fmt.Errorf("missing isin")
	}
	if listing.Symbol == "" {
		return Listing{}, fmt.Errorf("missing symbol for %s", listing.ISIN)
	}
	if listing.Currency == "" {
		return Listing{}, fmt.Errorf("missing currency for %s", listing.ISIN)
	}
	if listing.SaxoUIC < 0 || listing.IBKRConID < 0 || listing.IGShareSize < 0 {
		return Listing{}, fmt.Errorf("negative broker id for %s", listing.ISIN)
	}
	if listing.SaxoUIC > 0 && listing.SaxoAssetType == "" {
		return Listing{}, fmt.Errorf("missing saxo asset type for %s", listing.ISIN)
	}
	return listing, nil
}

// Get returns the listing for an ISIN.
func (b *Book) Get(isin string) (Listing, bool) {
	if b == nil {
		return Listing{}, false
	}
	row, ok := b.byISIN[normalizeISIN(isin)]
	return row, ok
}

// List returns the rows in file order.
func (b *Book) List() []Listing {
	if b == nil {
		return nil
	}
	out := make([]Listing, len(b.rows))
	copy(out, b.rows)
	return out
}
