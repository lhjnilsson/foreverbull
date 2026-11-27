package trading

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/lhjnilsson/foreverbull/pkg/finance/internal/mapping"
	"github.com/lhjnilsson/foreverbull/pkg/finance/internal/priceunit"
	"github.com/lhjnilsson/foreverbull/pkg/finance/supplier"
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

// Listing maps one ISIN on one venue to broker ids. The same ISIN on another
// MIC is another listing. Currency and PriceUnit are both stored; a price
// unit is not derived from the currency. ValidFrom and ValidTo are a
// half-open window. A zero ValidFrom and a nil ValidTo cover every day.
// Empty or zero broker ids mean that broker cannot trade the row.
type Listing struct {
	ISIN          string
	MIC           string
	Symbol        string
	Currency      string
	PriceUnit     string
	ValidFrom     time.Time
	ValidTo       *time.Time
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
	byISIN map[string][]Listing
}

type listingJSON struct {
	ISIN          string `json:"isin"`
	MIC           string `json:"mic"`
	Symbol        string `json:"symbol"`
	Currency      string `json:"currency"`
	PriceUnit     string `json:"price_unit"`
	ValidFrom     string `json:"valid_from"`
	ValidTo       string `json:"valid_to"`
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
		book, err := parseListings(sampleListingsJSON)
		if err != nil {
			return nil, err
		}
		learnListingSymbols(book)
		return book, nil
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, listingsFileError(path, fmt.Errorf("the file could not be read (%w).", err)) //nolint:staticcheck // ST1005: the problem ends with a full stop
	}
	book, err := parseListings(raw)
	if err != nil {
		return nil, listingsFileError(path, err)
	}
	learnListingSymbols(book)
	return book, nil
}

func learnListingSymbols(book *Book) {
	if book == nil {
		return
	}
	symbols := make([]string, 0, len(book.rows))
	for _, row := range book.rows {
		symbols = append(symbols, row.Symbol)
	}
	mapping.Process().Learn(symbols...)
}

func listingsFileError(path string, err error) error {
	// %w keeps the row or read error available to errors.Is and errors.As.
	// The verb is not at the end: the restart sentence follows it.
	return fmt.Errorf("Couldn't load the trading listings in %s: %w Fix the file, then restart foreverbull with `uv run fbenv stop && uv run fbenv start`.", path, err) //nolint:staticcheck // ST1005: agreed user-facing copy ends with a period
}

// listedProblems is every bad row, in file order. A row problem has no full
// stop of its own. Error joins them with "; " and adds one stop at the end.
type listedProblems struct {
	rows []error
}

func (e listedProblems) Error() string {
	parts := make([]string, len(e.rows))
	for i, row := range e.rows {
		parts[i] = row.Error()
	}
	return strings.Join(parts, "; ") + "."
}

func (e listedProblems) Unwrap() []error { return e.rows }

func parseListings(raw []byte) (*Book, error) {
	var decoded []listingJSON
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return nil, fmt.Errorf("the file is not JSON (%w).", err) //nolint:staticcheck // ST1005: the problem ends with a full stop
	}
	if len(decoded) == 0 {
		return nil, fmt.Errorf("the file is empty.") //nolint:staticcheck // ST1005: the problem ends with a full stop
	}

	book := &Book{
		rows:   make([]Listing, 0, len(decoded)),
		byISIN: make(map[string][]Listing, len(decoded)),
	}
	var problems []error
	for i, row := range decoded {
		listing, err := normalizeListing(row, i+1)
		if err != nil {
			problems = append(problems, err)
			continue
		}
		overlapped := false
		for _, prev := range book.byISIN[listing.ISIN] {
			if prev.MIC == listing.MIC && listingsOverlap(prev, listing) {
				problems = append(problems, overlapProblem(i+1, listing))
				overlapped = true
				break
			}
		}
		if overlapped {
			continue
		}
		book.byISIN[listing.ISIN] = append(book.byISIN[listing.ISIN], listing)
		book.rows = append(book.rows, listing)
	}
	if len(problems) > 0 {
		return nil, listedProblems{rows: problems}
	}
	return book, nil
}

func rowHas(n int, isin, what string) error {
	return fmt.Errorf("row %d (%s) has %s", n, isin, what)
}

func overlapProblem(n int, listing Listing) error {
	if listing.MIC == "" {
		return fmt.Errorf("row %d (%s, no mic) overlaps the dates of an earlier row for the same ISIN with no mic", n, listing.ISIN)
	}
	return fmt.Errorf("row %d (%s on %s) overlaps the dates of an earlier row for the same ISIN and exchange", n, listing.ISIN, listing.MIC)
}

func normalizeListing(row listingJSON, n int) (Listing, error) {
	listing := Listing{
		ISIN:          normalizeISIN(row.ISIN),
		MIC:           strings.ToUpper(strings.TrimSpace(row.MIC)),
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
		return Listing{}, fmt.Errorf("row %d has no ISIN", n)
	}
	if listing.Symbol == "" {
		return Listing{}, rowHas(n, listing.ISIN, "no symbol")
	}
	if listing.Currency == "" {
		return Listing{}, rowHas(n, listing.ISIN, "no currency")
	}
	written := strings.TrimSpace(row.PriceUnit)
	unit, err := priceunit.Canonical(listing.Currency, written)
	if err != nil {
		return Listing{}, rowHas(n, listing.ISIN, fmt.Sprintf("price unit '%s', which isn't a unit of %s", written, listing.Currency))
	}
	listing.PriceUnit = unit
	from, err := parseListingDate(row.ValidFrom)
	if err != nil {
		return Listing{}, rowHas(n, listing.ISIN, fmt.Sprintf("valid_from '%s', which isn't a date", strings.TrimSpace(row.ValidFrom)))
	}
	listing.ValidFrom = from
	if strings.TrimSpace(row.ValidTo) != "" {
		end, err := parseListingDate(row.ValidTo)
		if err != nil {
			return Listing{}, rowHas(n, listing.ISIN, fmt.Sprintf("valid_to '%s', which isn't a date", strings.TrimSpace(row.ValidTo)))
		}
		if !listing.ValidFrom.IsZero() && !end.After(listing.ValidFrom) {
			return Listing{}, rowHas(n, listing.ISIN, "valid_to on or before valid_from")
		}
		listing.ValidTo = &end
	}
	if listing.SaxoUIC < 0 {
		return Listing{}, rowHas(n, listing.ISIN, "a negative saxo_uic")
	}
	if listing.IBKRConID < 0 {
		return Listing{}, rowHas(n, listing.ISIN, "a negative ibkr_conid")
	}
	if listing.IGShareSize < 0 {
		return Listing{}, rowHas(n, listing.ISIN, "a negative ig_share_size")
	}
	if listing.SaxoUIC > 0 && listing.SaxoAssetType == "" {
		return Listing{}, rowHas(n, listing.ISIN, "saxo_uic and no saxo_asset_type")
	}
	return listing, nil
}

func parseListingDate(raw string) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, nil
	}
	parsed, err := time.Parse(time.DateOnly, raw)
	if err != nil {
		return time.Time{}, fmt.Errorf("not a date")
	}
	return parsed, nil
}

// listingsOverlap reports whether two listings of one ISIN and MIC share a day.
// A zero start is unbounded. A nil end is open. The windows are half-open.
func listingsOverlap(a, b Listing) bool {
	if !a.ValidFrom.IsZero() && b.ValidTo != nil && !b.ValidTo.After(a.ValidFrom) {
		return false
	}
	if !b.ValidFrom.IsZero() && a.ValidTo != nil && !a.ValidTo.After(b.ValidFrom) {
		return false
	}
	return true
}

func (l Listing) covers(on time.Time) bool {
	on = dateOnly(on)
	if !l.ValidFrom.IsZero() && on.Before(dateOnly(l.ValidFrom)) {
		return false
	}
	if l.ValidTo != nil && !on.Before(dateOnly(*l.ValidTo)) {
		return false
	}
	return true
}

// Get returns the listing for an ISIN that has exactly one row. Two venues
// are not one listing: the caller names the MIC.
func (b *Book) Get(isin string) (Listing, bool) {
	if b == nil {
		return Listing{}, false
	}
	rows := b.byISIN[normalizeISIN(isin)]
	if len(rows) != 1 {
		return Listing{}, false
	}
	return rows[0], true
}

// ListingAt returns the one listing of isin on mic that covers on. An empty
// mic matches a listing whose venue was not stored. Two rows is not a match.
func (b *Book) ListingAt(isin, mic string, on time.Time) (Listing, bool) {
	if b == nil {
		return Listing{}, false
	}
	mic = strings.ToUpper(strings.TrimSpace(mic))
	var found []Listing
	for _, row := range b.byISIN[normalizeISIN(isin)] {
		if mic != "" && row.MIC != mic {
			continue
		}
		if mic == "" && row.MIC != "" {
			continue
		}
		if !row.covers(on) {
			continue
		}
		found = append(found, row)
	}
	if len(found) != 1 {
		return Listing{}, false
	}
	return found[0], true
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

// listingsByISIN keeps every row. The order date chooses among them.
func listingsByISIN(rows []Listing) map[string][]Listing {
	out := make(map[string][]Listing, len(rows))
	for _, row := range rows {
		out[row.ISIN] = append(out[row.ISIN], row)
	}
	return out
}

func exchangeList(mics []string) string {
	switch len(mics) {
	case 0:
		return ""
	case 1:
		return mics[0]
	case 2:
		return mics[0] + " and " + mics[1]
	default:
		return strings.Join(mics[:len(mics)-1], ", ") + " and " + mics[len(mics)-1]
	}
}

func venueNames(rows []Listing) []string {
	names := make([]string, 0, len(rows))
	for _, row := range rows {
		if row.MIC == "" {
			continue
		}
		seen := false
		for _, name := range names {
			if name == row.MIC {
				seen = true
				break
			}
		}
		if !seen {
			names = append(names, row.MIC)
		}
	}
	return names
}

// ambiguousListing is the order refusal for an ISIN on more than one
// exchange. Error is the sentence a person reads. Unwrap is
// supplier.ErrNotTradable, so errors.Is still matches.
type ambiguousListing struct {
	sentence string
}

func (e ambiguousListing) Error() string { return e.sentence }

func (e ambiguousListing) Unwrap() error { return supplier.ErrNotTradable }

const orderRestart = "restart foreverbull with `uv run fbenv stop && uv run fbenv start`, then deploy the strategy again with the same options."

func severalExchanges(isin, names string) string {
	return fmt.Sprintf("%s is listed on more than one exchange in the trading listings (%s), so foreverbull can't tell where to place the order. Keep one of them in the listings file, %s", isin, names, orderRestart)
}

func severalListings(isin string) string {
	return fmt.Sprintf("%s has more than one listing in the trading listings, so foreverbull can't tell where to place the order. Keep one of them in the listings file, %s", isin, orderRestart)
}

// orderListingNow uses the day the order is placed. An order has no date.
func orderListingNow(rows []Listing, isin string) (Listing, error) {
	return orderListing(rows, isin, time.Now().UTC())
}

// orderListing returns the one listing of isin whose window covers on.
// A row with no MIC is not a venue. More than one venue on that day is
// refused. More than one row with no venue, and no venue row that day, is
// refused without naming a venue.
func orderListing(rows []Listing, isin string, on time.Time) (Listing, error) {
	var withVenue, without []Listing
	for _, row := range rows {
		if !row.covers(on) {
			continue
		}
		if row.MIC == "" {
			without = append(without, row)
			continue
		}
		withVenue = append(withVenue, row)
	}
	if len(withVenue) > 1 {
		return Listing{}, ambiguousListing{sentence: severalExchanges(isin, exchangeList(venueNames(withVenue)))}
	}
	if len(withVenue) == 1 {
		return withVenue[0], nil
	}
	if len(without) > 1 {
		return Listing{}, ambiguousListing{sentence: severalListings(isin)}
	}
	if len(without) == 1 {
		return without[0], nil
	}
	return Listing{}, fmt.Errorf("isin %s: %w", isin, supplier.ErrNotTradable)
}
