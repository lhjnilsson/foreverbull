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
	"path"
	"path/filepath"
	"regexp"
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

var holdingsDateListPattern = regexp.MustCompile(`initAsOfDateList"\s*:\s*\{\s*"all"\s*:\s*\[([0-9,]+)\]`)

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
// Unpublished as-of dates (empty aaData) are not cached; the previous 6 calendar days are tried.
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
		if errors.Is(err, errHoldingsPage) {
			return nil, e.holdingsPageError(etf, baseurl, asOf)
		}
		return assets, err
	}
	for i := 0; i <= snapshotLookbackDays; i++ {
		day := asOf.AddDate(0, 0, -i)
		if i > 0 {
			if body, ok, err := e.loadCached(etf, day); err != nil {
				return nil, err
			} else if ok {
				assets, published, err := parseHoldings(body)
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
			return nil, e.holdingsPageError(etf, baseurl, asOf)
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

func (e *ISharesETFLoopup) holdingsPageError(etf, baseurl string, asOf time.Time) error {
	when := dateOnly(asOf).Format("2006-01-02")
	dates, err := e.publishedHoldingDates(baseurl)
	if err != nil || len(dates) == 0 {
		return fmt.Errorf("iShares has no holdings file for %s on %s. The site returned a web page instead of holdings. Choose another date range, or pass a symbol file with --file", etf, when)
	}
	formatted := make([]string, len(dates))
	for i, d := range dates {
		formatted[i] = d.Format("2006-01-02")
	}
	return fmt.Errorf("iShares has no holdings file for %s on %s. Published holdings dates: %s. Choose a range that includes one of those, or pass a symbol file with --file",
		etf, when, strings.Join(formatted, ", "))
}

func (e *ISharesETFLoopup) publishedHoldingDates(baseurl string) ([]time.Time, error) {
	pageURL, err := productPageURL(baseurl)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodGet, pageURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := e.httpClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("product page status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return parsePublishedHoldingDates(body)
}

func productPageURL(baseurl string) (string, error) {
	u, err := url.Parse(baseurl)
	if err != nil {
		return "", err
	}
	u.Path = path.Dir(u.Path)
	u.RawQuery = ""
	u.Fragment = ""
	return u.String(), nil
}

func parsePublishedHoldingDates(body []byte) ([]time.Time, error) {
	text := strings.ReplaceAll(string(body), "&quot;", `"`)
	match := holdingsDateListPattern.FindStringSubmatch(text)
	if match == nil {
		return nil, errors.New("published holdings dates were not on the product page")
	}
	parts := strings.Split(match[1], ",")
	dates := make([]time.Time, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		asOf, err := time.Parse("20060102", part)
		if err != nil {
			return nil, fmt.Errorf("published holdings date %s: %w", part, err)
		}
		dates = append(dates, dateOnly(asOf))
	}
	sort.Slice(dates, func(i, j int) bool { return dates[i].Before(dates[j]) })
	return dates, nil
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
