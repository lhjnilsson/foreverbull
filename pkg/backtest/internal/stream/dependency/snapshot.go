package dependency

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"
	"unicode"

	"github.com/lhjnilsson/foreverbull/internal/usercopy"
)

type snapshotCache interface {
	load(etf string, asOf time.Time) ([]byte, bool, error)
	store(etf string, asOf time.Time, body []byte) error
	list(etf string, start, end time.Time) ([]time.Time, error)
	// folder is where the cache keeps etf's files, for the user's sentence.
	folder(etf string) string
}

// SnapshotCacheError is a read or write of the ETF holdings cache that
// failed. Its text is the user's sentence with a plain cause; Err keeps the
// file system error for the log.
type SnapshotCacheError struct {
	Dir   string
	Write bool
	Err   error
}

func (e *SnapshotCacheError) Error() string {
	if e.Write {
		return usercopy.SnapshotCacheWriteFailed(e.Dir, plainCacheCause(e.Err, true))
	}
	return usercopy.SnapshotCacheReadFailed(e.Dir, plainCacheCause(e.Err, false))
}

func (e *SnapshotCacheError) Unwrap() error { return e.Err }

// plainCacheCause says in plain words why the cache folder failed.
func plainCacheCause(err error, write bool) string {
	switch {
	case errors.Is(err, fs.ErrPermission) && write:
		return "foreverbull isn't allowed to write there"
	case errors.Is(err, fs.ErrPermission):
		return "foreverbull isn't allowed to read it"
	case errors.Is(err, syscall.ENOSPC):
		return "the disk is full"
	case write:
		return "the file couldn't be written"
	default:
		return "the folder couldn't be read"
	}
}

type fsSnapshotCache struct {
	dir string
}

func (c *fsSnapshotCache) folder(etf string) string {
	if c == nil {
		return ""
	}
	return filepath.Join(c.dir, etfCacheSlug(etf))
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

// list returns the cached as-of dates in [start, end]. A missing ETF
// directory is an empty cache. Any other read error is returned: the cached
// dates join the sampled ones, so a dropped list would change the universe.
// A file whose name isn't YYYYMMDD.json was not written here and is no as-of.
func (c *fsSnapshotCache) list(etf string, start, end time.Time) ([]time.Time, error) {
	if c == nil || c.dir == "" {
		return nil, nil
	}
	dir := filepath.Join(c.dir, etfCacheSlug(etf))
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
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
	return dates, nil
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

const (
	cellString = "string"
	cellNull   = "null"
	cellShort  = "short"
	cellBad    = "bad"
	cellAbsent = "absent"
)

// unusableSnapshot is the create failure for one iShares holdings list.
// The sentence names every row and column that can't be used. Callers return
// it before the backtest is stored.
func unusableSnapshot(etf string, asOf time.Time, problems []usercopy.HoldingsFault) error {
	if len(problems) == 0 {
		return nil
	}
	return errors.New(usercopy.HoldingsListUnusable(etf, asOf, problems))
}

func labelFrom(symbol, symbolKind, name, nameKind string) string {
	if symbolKind == cellString && symbol != "" {
		return symbol
	}
	if nameKind == cellString && name != "" {
		return name
	}
	return ""
}

func aaCell(row []interface{}, index int) (string, string) {
	if index >= len(row) || row[index] == nil {
		return "", cellNull
	}
	text, ok := row[index].(string)
	if !ok {
		return "", cellBad
	}
	return text, cellString
}

// usableISIN is false for a missing, blank, or "-" ISIN. "-" is the placeholder
// iShares uses when it has no ISIN.
func usableISIN(isin, kind string) bool {
	if kind != cellString {
		return false
	}
	isin = strings.TrimSpace(isin)
	return isin != "" && isin != "-"
}

// holdingFromAARow classifies one aaData row. A class other than Equity is
// left out. A null sector is allowed. A row shorter than 9 cells couldn't be
// read, even when the ISIN cell is the one that's missing. keep is true only
// for an Equity row with an ISIN.
func holdingFromAARow(row []interface{}, rowNum int) (Asset, usercopy.HoldingsFault, bool) {
	symbol, symbolKind := aaCell(row, 0)
	name, nameKind := aaCell(row, 1)
	sector, sectorKind := aaCell(row, 2)
	assetType, typeKind := aaCell(row, 3)
	isin, isinKind := aaCell(row, 8)
	if typeKind == cellString && assetType != "Equity" {
		return Asset{}, usercopy.HoldingsFault{}, false
	}
	label := labelFrom(symbol, symbolKind, name, nameKind)
	fault := usercopy.HoldingsFault{Row: rowNum, Label: label, Kind: usercopy.HoldingsUnread}
	if len(row) < 9 {
		return Asset{}, fault, false
	}
	if symbolKind != cellString || symbol == "" || nameKind != cellString || typeKind != cellString || sectorKind == cellBad || isinKind == cellBad {
		return Asset{}, fault, false
	}
	if !usableISIN(isin, isinKind) {
		fault.Kind = usercopy.HoldingsNoISIN
		return Asset{}, fault, false
	}
	if sectorKind != cellString {
		sector = ""
	}
	return Asset{
		Symbol:    symbol,
		Name:      name,
		Sector:    sector,
		AssetType: assetType,
		ISIN:      isin,
	}, usercopy.HoldingsFault{}, true
}

// errHoldingsPage means the holdings URL returned the product web page.
// iShares does that for as-of dates it does not publish.
var errHoldingsPage = errors.New("iShares returned a web page instead of holdings")

func parseHoldings(body []byte, etf string, asOf time.Time) (assets []Asset, published bool, err error) {
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
	var problems []usercopy.HoldingsFault
	for i, row := range data.Data {
		asset, problem, keep := holdingFromAARow(row, i+1)
		if problem.Kind != "" {
			problems = append(problems, problem)
			continue
		}
		if keep {
			assets = append(assets, asset)
		}
	}
	if fail := unusableSnapshot(etf, asOf, problems); fail != nil {
		return nil, false, fail
	}
	return assets, true, nil
}

const productDataHoldingsURL = "https://www.ishares.com/varnish-api/blk-one01-product-data/product-data/api/v2/get-product-data"

func holdingsURL(baseurl string, asOf time.Time) (string, error) {
	if resolved, ok, err := productDataHoldingsRequest(baseurl, asOf); ok || err != nil {
		return resolved, err
	}
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

// productDataHoldingsRequest is the holdings call the iShares product page uses.
// The old *.ajax?fileType=json URL now returns the HTML product page for every as-of.
func productDataHoldingsRequest(baseurl string, asOf time.Time) (string, bool, error) {
	u, err := url.Parse(baseurl)
	if err != nil {
		return "", false, fmt.Errorf("error creating request: %w", err)
	}
	host := strings.ToLower(u.Hostname())
	if host != "www.ishares.com" && host != "ishares.com" {
		return "", false, nil
	}
	portfolioID := ""
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	for i, part := range parts {
		if part == "products" && i+1 < len(parts) && isDigits(parts[i+1]) {
			portfolioID = parts[i+1]
			break
		}
	}
	if portfolioID == "" {
		return "", false, nil
	}
	q := url.Values{}
	q.Set("appSubType", "ISHARES")
	q.Set("appType", "PRODUCT_PAGE")
	q.Set("component", "holdings.all")
	q.Set("locale", "en_US")
	q.Set("portfolioId", portfolioID)
	q.Set("targetSite", "us-ishares")
	q.Set("userType", "individual")
	q.Set("excludeContent", "true")
	q.Set("includeConfig", "true")
	q.Set("asOfDate", dateOnly(asOf).Format("20060102"))
	return productDataHoldingsURL + "?" + q.Encode(), true, nil
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func (e *ISharesETFLoopup) fetchHoldings(etf, baseurl string, asOf time.Time) ([]Asset, []byte, bool, error) {
	rawURL, err := holdingsURL(baseurl, asOf)
	if err != nil {
		return nil, nil, false, err
	}
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, nil, false, fmt.Errorf("error creating request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("x-application-id", "pp-ui-csr")
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
	if isProductDataHoldings(body) {
		assets, gotAsOf, err := parseProductDataHoldings(body, etf, asOf, true)
		if err != nil {
			return nil, nil, false, err
		}
		want := dateOnly(asOf).Format("20060102")
		if gotAsOf != want || len(assets) == 0 {
			return nil, nil, false, nil
		}
		return assets, body, true, nil
	}
	assets, published, err := parseHoldings(body, etf, asOf)
	if err != nil {
		return nil, nil, false, err
	}
	return assets, body, published, nil
}

func parseCachedHoldings(body []byte, etf string, asOf time.Time) ([]Asset, bool, error) {
	if isProductDataHoldings(body) {
		assets, _, err := parseProductDataHoldings(body, etf, asOf, false)
		if err != nil {
			return nil, false, err
		}
		return assets, len(assets) > 0, nil
	}
	return parseHoldings(body, etf, asOf)
}

func isProductDataHoldings(body []byte) bool {
	return bytes.Contains(body, []byte(`"componentsByNameMap"`))
}

// parseProductDataHoldings reads one product-data holdings reply.
// matchAsOf is set on a live fetch. An as-of that is not the requested day
// returns before the columns are read. A reply whose columns are all null
// is that same unpublished day. Both are nil assets and a nil error, so the
// caller can walk back. A cached file is read with matchAsOf false: it was
// stored under the requested day, and its own as-of can be the earlier
// session it came from.
func parseProductDataHoldings(body []byte, etf string, asOf time.Time, matchAsOf bool) ([]Asset, string, error) {
	var payload struct {
		Components map[string]struct {
			Containers map[string]struct {
				Points map[string]struct {
					Value json.RawMessage `json:"value"`
				} `json:"dataPointsByNameMap"`
			} `json:"containersByNameMap"`
		} `json:"componentsByNameMap"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, "", fmt.Errorf("iShares holdings were not valid JSON: %w", err)
	}
	points := payload.Components["holdings"].Containers["all"].Points
	publishedAsOf := rawScalar(points["asOfDate"].Value)
	if matchAsOf && publishedAsOf != dateOnly(asOf).Format("20060102") {
		return nil, publishedAsOf, nil
	}

	names := []string{"ticker", "issueName", "sectorName", "assetClass", "isin"}
	if columnsBlank(points["ticker"].Value, points["issueName"].Value, points["sectorName"].Value, points["assetClass"].Value, points["isin"].Value) {
		return nil, publishedAsOf, nil
	}
	decoded := make(map[string][]json.RawMessage, len(names))
	decodedOK := make(map[string]bool, len(names))
	columnProblems := make([]usercopy.HoldingsFault, 0)
	for _, name := range names {
		items, ok := decodeColumn(points[name].Value)
		decoded[name] = items
		decodedOK[name] = ok
		if !ok {
			columnProblems = append(columnProblems, usercopy.HoldingsFault{Column: name, Kind: usercopy.HoldingsUnread})
		}
	}
	assets, rowProblems := equityRows(decoded, decodedOK)
	if fail := unusableSnapshot(etf, asOf, append(rowProblems, columnProblems...)); fail != nil {
		return nil, publishedAsOf, fail
	}
	return assets, publishedAsOf, nil
}

func columnsBlank(values ...json.RawMessage) bool {
	for _, raw := range values {
		if !rawBlank(raw) {
			return false
		}
	}
	return true
}

// equityRows reads every column's rows, using the longest column. A missing
// ticker, name, asset class, or ISIN cell couldn't be read. sectorName may
// be null or short and is not required. A class other than Equity is left
// out. A blank or null ticker, and a null asset class, couldn't be read.
func equityRows(cols map[string][]json.RawMessage, okCol map[string]bool) ([]Asset, []usercopy.HoldingsFault) {
	n := 0
	for _, name := range []string{"ticker", "issueName", "sectorName", "assetClass", "isin"} {
		if okCol[name] && len(cols[name]) > n {
			n = len(cols[name])
		}
	}
	assets := make([]Asset, 0, n)
	var problems []usercopy.HoldingsFault
	for i := 0; i < n; i++ {
		asset, problem, keep := equityAt(cols, okCol, i)
		if problem.Kind != "" {
			problems = append(problems, problem)
			continue
		}
		if keep {
			assets = append(assets, asset)
		}
	}
	return assets, problems
}

func equityAt(cols map[string][]json.RawMessage, okCol map[string]bool, index int) (Asset, usercopy.HoldingsFault, bool) {
	ticker, tickerKind := locatedCell(cols, okCol, "ticker", index)
	class, classKind := locatedCell(cols, okCol, "assetClass", index)
	name, nameKind := locatedCell(cols, okCol, "issueName", index)
	sector, sectorKind := locatedCell(cols, okCol, "sectorName", index)
	isin, isinKind := locatedCell(cols, okCol, "isin", index)
	if classKind == cellString && class != "Equity" {
		return Asset{}, usercopy.HoldingsFault{}, false
	}
	label := labelFrom(ticker, tickerKind, name, nameKind)
	rowNum := index + 1
	if rowUnreadable(ticker, tickerKind, classKind, nameKind, sectorKind, isinKind) {
		return Asset{}, usercopy.HoldingsFault{Row: rowNum, Label: label, Kind: usercopy.HoldingsUnread}, false
	}
	if classKind != cellString {
		return Asset{}, usercopy.HoldingsFault{}, false
	}
	if isinKind == cellAbsent {
		return Asset{}, usercopy.HoldingsFault{}, false
	}
	if !usableISIN(isin, isinKind) {
		return Asset{}, usercopy.HoldingsFault{Row: rowNum, Label: label, Kind: usercopy.HoldingsNoISIN}, false
	}
	if sectorKind != cellString {
		sector = ""
	}
	if nameKind != cellString {
		name = ""
	}
	return Asset{
		Symbol:    ticker,
		Name:      name,
		Sector:    sector,
		AssetType: "Equity",
		ISIN:      isin,
	}, usercopy.HoldingsFault{}, true
}

func rowUnreadable(ticker, tickerKind, classKind, nameKind, sectorKind, isinKind string) bool {
	if tickerKind != cellAbsent && (tickerKind != cellString || ticker == "") {
		return true
	}
	if classKind == cellNull || classKind == cellShort || classKind == cellBad {
		return true
	}
	if nameKind == cellShort || nameKind == cellBad {
		return true
	}
	if sectorKind == cellBad {
		return true
	}
	return isinKind == cellShort || isinKind == cellBad
}

func locatedCell(cols map[string][]json.RawMessage, okCol map[string]bool, name string, index int) (string, string) {
	if !okCol[name] {
		return "", cellAbsent
	}
	items := cols[name]
	if index >= len(items) {
		return "", cellShort
	}
	return rawCell(items[index])
}

func decodeColumn(raw json.RawMessage) ([]json.RawMessage, bool) {
	if rawBlank(raw) {
		return nil, false
	}
	var items []json.RawMessage
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, false
	}
	return items, true
}

func rawCell(raw json.RawMessage) (string, string) {
	if rawBlank(raw) {
		return "", cellNull
	}
	text, ok := scalarText(raw)
	if !ok {
		return "", cellBad
	}
	return text, cellString
}

func rawBlank(raw json.RawMessage) bool {
	trimmed := bytes.TrimSpace(raw)
	return len(trimmed) == 0 || string(trimmed) == "null"
}

func rawScalar(raw json.RawMessage) string {
	text, ok := scalarText(raw)
	if !ok {
		return ""
	}
	return text
}

func scalarText(raw json.RawMessage) (string, bool) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || string(raw) == "null" {
		return "", true
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return text, true
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var number json.Number
	if err := dec.Decode(&number); err != nil {
		return "", false
	}
	if dec.Decode(new(json.RawMessage)) != io.EOF {
		return "", false
	}
	return number.String(), true
}

func (e *ISharesETFLoopup) loadCached(etf string, asOf time.Time) ([]byte, bool, error) {
	if e.cache == nil {
		return nil, false, nil
	}
	body, ok, err := e.cache.load(etf, asOf)
	if err != nil {
		return nil, false, &SnapshotCacheError{Dir: e.cache.folder(etf), Err: err}
	}
	return body, ok, nil
}

func (e *ISharesETFLoopup) storeCached(etf string, asOf time.Time, body []byte) error {
	if e.cache == nil {
		return nil
	}
	if err := e.cache.store(etf, asOf, body); err != nil {
		return &SnapshotCacheError{Dir: e.cache.folder(etf), Write: true, Err: err}
	}
	return nil
}

// GetSnapshot returns Equity holdings (with ISIN) for etf as of asOf.
// A row that couldn't be read, an Equity row with no ISIN, or a column that
// couldn't be read fails this snapshot. The error names each of them.
// Empty aaData, a product-data as-of that is not this day, and a product-data
// reply whose columns are all null are unpublished and are not cached; the
// previous 6 calendar days are tried.
// An HTML product page means that day has no holdings file. It is not cached and is not an error.
// A sampled month that stays empty fails create; it is not filled from another list.
func (e *ISharesETFLoopup) GetSnapshot(etf string, asOf time.Time) ([]Asset, error) {
	baseurl, err := e.productURL(etf)
	if err != nil {
		return nil, err
	}
	asOf = dateOnly(asOf)
	if body, ok, err := e.loadCached(etf, asOf); err != nil {
		return nil, err
	} else if ok {
		assets, _, err := parseCachedHoldings(body, etf, asOf)
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
				assets, published, err := parseCachedHoldings(body, etf, day)
				if errors.Is(err, errHoldingsPage) {
					err = nil
					published = false
				}
				if err != nil {
					return nil, err
				}
				if published {
					if err := e.storeCached(etf, asOf, body); err != nil {
						return nil, err
					}
					return assets, nil
				}
			}
		}
		assets, body, published, err := e.fetchHoldings(etf, baseurl, day)
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

// Listing is the equity holdings published for one as-of in a create window.
// An empty Assets slice means that as-of had no holdings file.
type Listing struct {
	AsOf   time.Time
	Assets []Asset
}

// LoadSnapshotWindow loads each sampled as-of in [start, end], plus the seed:
// the latest listing published on or before start (§6.2 of universe.md). The
// seed keeps its own as-of, which can be before start. A month in the window
// with no holdings file fails, and so does a start with nothing published on
// or before it. Nothing is copied from another list. The error names the ETF
// and the window only when nothing parsed.
func LoadSnapshotWindow(scraper ETFScraper, etf string, start, end time.Time) ([]Listing, error) {
	start = dateOnly(start)
	end = dateOnly(end)
	dates, err := scraper.ListSnapshotDates(etf, start, end)
	if err != nil {
		return nil, fmt.Errorf("error listing snapshot dates: %w", err)
	}
	seed, seeded, err := loadSeedListing(scraper, etf, start)
	if err != nil {
		return nil, err
	}
	listings := make([]Listing, 0, len(dates)+1)
	if seeded {
		listings = append(listings, seed)
	}
	for _, asOf := range dates {
		asOf = dateOnly(asOf)
		if seeded && asOf.Equal(seed.AsOf) {
			continue
		}
		assets, err := scraper.GetSnapshot(etf, asOf)
		if err != nil {
			return nil, err
		}
		listings = append(listings, Listing{AsOf: asOf, Assets: assets})
	}
	sort.Slice(listings, func(i, j int) bool { return listings[i].AsOf.Before(listings[j].AsOf) })
	if err := RejectUnpublishedListings(etf, start, end, listings); err != nil {
		return nil, err
	}
	return listings, nil
}

// loadSeedListing returns the latest listing published on or before start.
// iShares serves a holdings file for any past as-of, so the month-ends before
// start are read the same way as the ones in the window. The walk is newest
// first and stops at the first published file; seedLookbackMonths bounds it,
// so an ETF that launched after start costs at most that many reads.
func loadSeedListing(scraper ETFScraper, etf string, start time.Time) (Listing, bool, error) {
	year, month, _ := start.Date()
	from := time.Date(year, month-seedLookbackMonths, 1, 0, 0, 0, 0, time.UTC)
	dates, err := scraper.ListSnapshotDates(etf, from, start)
	if err != nil {
		return Listing{}, false, fmt.Errorf("error listing snapshot dates before the start: %w", err)
	}
	candidates := make([]time.Time, 0, len(dates))
	for _, asOf := range dates {
		asOf = dateOnly(asOf)
		if asOf.Before(from) || asOf.After(start) {
			continue
		}
		candidates = append(candidates, asOf)
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].After(candidates[j]) })
	for _, asOf := range candidates {
		assets, err := scraper.GetSnapshot(etf, asOf)
		if err != nil {
			return Listing{}, false, err
		}
		if len(assets) > 0 {
			return Listing{AsOf: asOf, Assets: assets}, true, nil
		}
	}
	return Listing{}, false, nil
}

// RejectUnpublishedListings fails create when no holdings list was published
// on or before start, or when a sampled as-of in the window has no file.
// It does not copy holdings onto those dates. Nothing published in the whole
// window uses ETFNoHoldingsInWindow. A gap before the first list after the
// start is not named again: that list is already the new start.
func RejectUnpublishedListings(etf string, start, end time.Time, listings []Listing) error {
	start = dateOnly(start)
	end = dateOnly(end)
	published := 0
	hasOnOrBefore := false
	var firstAfter time.Time
	missing := make([]time.Time, 0)
	for i := range listings {
		asOf := dateOnly(listings[i].AsOf)
		listings[i].AsOf = asOf
		if len(listings[i].Assets) > 0 {
			published++
			if !asOf.After(start) {
				hasOnOrBefore = true
				continue
			}
			if firstAfter.IsZero() || asOf.Before(firstAfter) {
				firstAfter = asOf
			}
			continue
		}
		if !asOf.Before(start) && !asOf.After(end) {
			missing = append(missing, asOf)
		}
	}
	if published == 0 || (!hasOnOrBefore && firstAfter.IsZero()) {
		return errors.New(usercopy.ETFNoHoldingsInWindow(etf, start, end))
	}
	sentences := make([]string, 0, 2)
	if !hasOnOrBefore {
		sentences = append(sentences, usercopy.ETFNoHoldingsAtStart(etf, start, firstAfter))
		missing = gapsOnOrAfter(missing, firstAfter)
	}
	if len(missing) > 0 {
		sentences = append(sentences, usercopy.ETFHoldingsGap(etf, missing))
	}
	if len(sentences) == 0 {
		return nil
	}
	return errors.New(strings.Join(sentences, " "))
}

// gapsOnOrAfter drops months a later start already leaves out. first is the
// first published list after the backtest start.
func gapsOnOrAfter(missing []time.Time, first time.Time) []time.Time {
	kept := make([]time.Time, 0, len(missing))
	for _, day := range missing {
		if !day.Before(first) {
			kept = append(kept, day)
		}
	}
	return kept
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
		var err error
		cached, err = e.cache.list(etf, start, end)
		if err != nil {
			return nil, &SnapshotCacheError{Dir: e.cache.folder(etf), Err: err}
		}
	}
	return mergeDates(monthEndDates(start, end), cached), nil
}
