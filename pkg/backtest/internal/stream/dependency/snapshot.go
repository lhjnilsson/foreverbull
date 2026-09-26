package dependency

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"
)

type snapshotCache interface {
	load(etf string, asOf time.Time) ([]byte, bool, error)
	store(etf string, asOf time.Time, body []byte) error
	list(etf string, start, end time.Time) []time.Time
}

type fsSnapshotCache struct {
	dir string
}

func (c *fsSnapshotCache) path(etf string, asOf time.Time) string {
	return filepath.Join(c.dir, etfCacheSlug(etf), dateOnly(asOf).Format("20060102")+".json")
}

func (c *fsSnapshotCache) load(etf string, asOf time.Time) ([]byte, bool, error) {
	if c == nil || c.dir == "" {
		return nil, false, nil
	}
	body, err := os.ReadFile(c.path(etf, asOf))
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return body, true, nil
}

func (c *fsSnapshotCache) store(etf string, asOf time.Time, body []byte) error {
	if c == nil || c.dir == "" {
		return nil
	}
	path := c.path(etf, asOf)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, body, 0o644)
}

func (c *fsSnapshotCache) list(etf string, start, end time.Time) []time.Time {
	if c == nil || c.dir == "" {
		return nil
	}
	entries, err := os.ReadDir(filepath.Join(c.dir, etfCacheSlug(etf)))
	if err != nil {
		return nil
	}
	start = dateOnly(start)
	end = dateOnly(end)
	var dates []time.Time
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := strings.TrimSuffix(entry.Name(), ".json")
		if name == entry.Name() {
			continue
		}
		asOf, err := time.Parse("20060102", name)
		if err != nil {
			continue
		}
		asOf = dateOnly(asOf)
		if asOf.Before(start) || asOf.After(end) {
			continue
		}
		dates = append(dates, asOf)
	}
	return dates
}

func etfCacheSlug(etf string) string {
	var b strings.Builder
	prevUnderscore := false
	for _, r := range etf {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			prevUnderscore = false
			continue
		}
		if !prevUnderscore {
			b.WriteByte('_')
			prevUnderscore = true
		}
	}
	return strings.Trim(b.String(), "_")
}

func dateOnly(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func monthEndDates(start, end time.Time) []time.Time {
	start = dateOnly(start)
	end = dateOnly(end)
	if end.Before(start) {
		return nil
	}
	var dates []time.Time
	year, month, _ := start.Date()
	for {
		last := time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC)
		if last.After(end) {
			break
		}
		if !last.Before(start) {
			dates = append(dates, last)
		}
		if month == time.December {
			year++
			month = time.January
		} else {
			month++
		}
	}
	return dates
}

func mergeDates(groups ...[]time.Time) []time.Time {
	seen := make(map[string]time.Time)
	for _, group := range groups {
		for _, d := range group {
			d = dateOnly(d)
			seen[d.Format("20060102")] = d
		}
	}
	dates := make([]time.Time, 0, len(seen))
	for _, d := range seen {
		dates = append(dates, d)
	}
	sort.Slice(dates, func(i, j int) bool { return dates[i].Before(dates[j]) })
	return dates
}

func trapBOM(fileBytes []byte) []byte {
	return bytes.Trim(fileBytes, "\xef\xbb\xbf")
}

func assetFromRow(row []interface{}) (Asset, bool) {
	if len(row) < 9 {
		return Asset{}, false
	}
	symbol, ok0 := row[0].(string)
	name, ok1 := row[1].(string)
	sector, ok2 := row[2].(string)
	assetType, ok3 := row[3].(string)
	isin, ok8 := row[8].(string)
	if !ok0 || !ok1 || !ok2 || !ok3 || !ok8 {
		return Asset{}, false
	}
	return Asset{
		Symbol:    symbol,
		Name:      name,
		Sector:    sector,
		AssetType: assetType,
		ISIN:      isin,
	}, true
}

// errHoldingsPage means the holdings URL returned the product web page.
// iShares does that for as-of dates it does not publish.
var errHoldingsPage = errors.New("iShares returned a web page instead of holdings")

func parseHoldings(body []byte) (assets []Asset, published bool, err error) {
	body = bytes.TrimSpace(trapBOM(body))
	if len(body) == 0 {
		return nil, false, errors.New("iShares returned an empty holdings response")
	}
	if body[0] == '<' {
		return nil, false, errHoldingsPage
	}
	var data Response
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, false, fmt.Errorf("iShares holdings were not valid JSON: %w", err)
	}
	if len(data.Data) == 0 {
		return nil, false, nil
	}
	assets = make([]Asset, 0, len(data.Data))
	for _, row := range data.Data {
		asset, ok := assetFromRow(row)
		if !ok || asset.AssetType != "Equity" || asset.ISIN == "" {
			continue
		}
		assets = append(assets, asset)
	}
	return assets, true, nil
}

func holdingsURL(baseurl string, asOf time.Time) (string, error) {
	u, err := url.Parse(baseurl)
	if err != nil {
		return "", fmt.Errorf("error creating request: %w", err)
	}
	q := u.Query()
	q.Set("tab", "all")
	q.Set("fileType", "json")
	q.Set("asOfDate", dateOnly(asOf).Format("20060102"))
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func (e *ISharesETFLoopup) fetchHoldings(baseurl string, asOf time.Time) ([]Asset, []byte, bool, error) {
	rawURL, err := holdingsURL(baseurl, asOf)
	if err != nil {
		return nil, nil, false, err
	}
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, nil, false, fmt.Errorf("error creating request: %w", err)
	}
	resp, err := e.httpClient().Do(req)
	if err != nil {
		return nil, nil, false, fmt.Errorf("error doing request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, nil, false, fmt.Errorf("error status code: %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, false, fmt.Errorf("error reading body: %w", err)
	}
	body = trapBOM(body)
	assets, published, err := parseHoldings(body)
	if err != nil {
		return nil, nil, false, err
	}
	return assets, body, published, nil
}

func (e *ISharesETFLoopup) loadCached(etf string, asOf time.Time) ([]byte, bool, error) {
	if e.cache == nil {
		return nil, false, nil
	}
	return e.cache.load(etf, asOf)
}

func (e *ISharesETFLoopup) storeCached(etf string, asOf time.Time, body []byte) error {
	if e.cache == nil {
		return nil
	}
	return e.cache.store(etf, asOf, body)
}

// GetSnapshot returns Equity holdings (with ISIN) for etf as of asOf.
// Empty aaData is unpublished and is not cached; the previous 6 calendar days are tried.
// An HTML product page means that day has no holdings file. It is not cached and is not an error:
// the caller fills that month from another published listing.
func (e *ISharesETFLoopup) GetSnapshot(etf string, asOf time.Time) ([]Asset, error) {
	baseurl, err := e.productURL(etf)
	if err != nil {
		return nil, err
	}
	asOf = dateOnly(asOf)
	if body, ok, err := e.loadCached(etf, asOf); err != nil {
		return nil, err
	} else if ok {
		assets, _, err := parseHoldings(body)
		if !errors.Is(err, errHoldingsPage) {
			return assets, err
		}
	}
	for i := 0; i <= snapshotLookbackDays; i++ {
		day := asOf.AddDate(0, 0, -i)
		if i > 0 {
			if body, ok, err := e.loadCached(etf, day); err != nil {
				return nil, err
			} else if ok {
				assets, published, err := parseHoldings(body)
				if errors.Is(err, errHoldingsPage) {
					err = nil
					published = false
				}
				if err != nil {
					return nil, err
				}
				if published {
					_ = e.storeCached(etf, asOf, body)
					return assets, nil
				}
			}
		}
		assets, body, published, err := e.fetchHoldings(baseurl, day)
		if errors.Is(err, errHoldingsPage) {
			// The product page is not a file for this day. The requested as-of is a gap;
			// a lookback day is just unpublished, so keep walking.
			if i == 0 {
				return []Asset{}, nil
			}
			continue
		}
		if err != nil {
			return nil, err
		}
		if !published {
			continue
		}
		if err := e.storeCached(etf, asOf, body); err != nil {
			return nil, err
		}
		return assets, nil
	}
	return []Asset{}, nil
}

// Listing is the equity holdings used for one as-of in a create window.
// A gap month keeps a copy of another listing. FilledFrom is that as-of.
// Leading is set when the copy comes from a later listing because nothing earlier was published.
type Listing struct {
	AsOf       time.Time
	Assets     []Asset
	FilledFrom time.Time
	Leading    bool
}

// LoadSnapshotWindow loads each sampled as-of in [start, end].
// A month with no holdings file is filled from the latest published listing on or
// before that date, or from the earliest listing after it when the window has not
// reached a published file yet. The error names the ETF and the window only when
// nothing in the range parsed.
func LoadSnapshotWindow(scraper ETFScraper, etf string, start, end time.Time) ([]Listing, error) {
	dates, err := scraper.ListSnapshotDates(etf, start, end)
	if err != nil {
		return nil, fmt.Errorf("error listing snapshot dates: %w", err)
	}
	listings := make([]Listing, 0, len(dates))
	for _, asOf := range dates {
		assets, err := scraper.GetSnapshot(etf, asOf)
		if err != nil {
			return nil, err
		}
		listings = append(listings, Listing{AsOf: dateOnly(asOf), Assets: assets})
	}
	sort.Slice(listings, func(i, j int) bool { return listings[i].AsOf.Before(listings[j].AsOf) })
	if err := fillListings(etf, start, end, listings); err != nil {
		return nil, err
	}
	return listings, nil
}

func fillListings(etf string, start, end time.Time, listings []Listing) error {
	published := make([]int, 0, len(listings))
	for i := range listings {
		listings[i].AsOf = dateOnly(listings[i].AsOf)
		if len(listings[i].Assets) > 0 {
			published = append(published, i)
		}
	}
	if len(published) == 0 {
		return fmt.Errorf("iShares has no holdings file for %s between %s and %s",
			etf, dateOnly(start).Format("2006-01-02"), dateOnly(end).Format("2006-01-02"))
	}
	sort.Slice(published, func(i, j int) bool {
		return listings[published[i]].AsOf.Before(listings[published[j]].AsOf)
	})
	for i := range listings {
		if len(listings[i].Assets) > 0 {
			continue
		}
		gap := listings[i].AsOf
		src := latestPublishedOnOrBefore(listings, published, gap)
		leading := false
		if src < 0 {
			src = earliestPublishedAfter(listings, published, gap)
			leading = src >= 0
		}
		if src < 0 {
			continue
		}
		listings[i].Assets = cloneAssets(listings[src].Assets)
		listings[i].FilledFrom = listings[src].AsOf
		listings[i].Leading = leading
	}
	return nil
}

func latestPublishedOnOrBefore(listings []Listing, published []int, gap time.Time) int {
	src := -1
	for _, j := range published {
		if listings[j].AsOf.After(gap) {
			break
		}
		src = j
	}
	return src
}

func earliestPublishedAfter(listings []Listing, published []int, gap time.Time) int {
	for _, j := range published {
		if listings[j].AsOf.After(gap) {
			return j
		}
	}
	return -1
}

func cloneAssets(in []Asset) []Asset {
	out := make([]Asset, len(in))
	copy(out, in)
	return out
}

// ListSnapshotDates returns last-calendar-day-of-month dates in [start, end],
// plus any as-of files already present in the snapshot cache.
func (e *ISharesETFLoopup) ListSnapshotDates(etf string, start, end time.Time) ([]time.Time, error) {
	if _, err := e.productURL(etf); err != nil {
		return nil, err
	}
	if start.IsZero() || end.IsZero() {
		return nil, fmt.Errorf("start and end dates are required")
	}
	var cached []time.Time
	if e.cache != nil {
		cached = e.cache.list(etf, start, end)
	}
	return mergeDates(monthEndDates(start, end), cached), nil
}
