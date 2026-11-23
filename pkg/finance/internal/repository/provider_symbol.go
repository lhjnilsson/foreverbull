package repository

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/rs/zerolog/log"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/lhjnilsson/foreverbull/internal/postgres"
	financepb "github.com/lhjnilsson/foreverbull/pkg/pb/finance"
)

// ProviderSymbolTable is which ticker a provider's bars for an instrument
// are stored under. ohlc stays keyed by that ticker. instrument_ohlc is the
// one series a read uses.
const ProviderSymbolTable = `
CREATE TABLE IF NOT EXISTS provider_symbol (
	instrument_id text NOT NULL REFERENCES instrument (id),
	provider text NOT NULL,
	symbol text NOT NULL,
	from_date date NOT NULL,
	to_date date,
	evidence text NOT NULL,
	PRIMARY KEY (instrument_id, provider, from_date),
	CONSTRAINT provider_symbol_provider_check CHECK (provider IN ('yahoo', 'stooq', 'alpaca')),
	CONSTRAINT provider_symbol_evidence_check CHECK (evidence IN ('download', 'snapshot', 'migration')),
	CONSTRAINT provider_symbol_no_overlap EXCLUDE USING gist (
		instrument_id WITH =,
		provider WITH =,
		daterange(from_date, to_date, '[)') WITH &&
	),
	CONSTRAINT provider_symbol_ticker_no_overlap EXCLUDE USING gist (
		provider WITH =,
		symbol WITH =,
		daterange(from_date, to_date, '[)') WITH &&
	)
);
`

const instrumentOHLCView = `
CREATE OR REPLACE VIEW instrument_ohlc AS
SELECT p.instrument_id, o.provider, o.time, o.open, o.high, o.low, o.close, o.volume, o.symbol
  FROM provider_symbol p
  JOIN ohlc o ON o.provider = p.provider AND o.symbol = p.symbol
 WHERE (o.time AT TIME ZONE 'UTC')::date >= p.from_date
   AND (p.to_date IS NULL OR (o.time AT TIME ZONE 'UTC')::date < p.to_date);
`

// MigrateProviderSymbol creates provider_symbol and instrument_ohlc.
// Calling it twice is safe. It does not rewrite ohlc.
func MigrateProviderSymbol(ctx context.Context, conn postgres.Query) error {
	if _, err := conn.Exec(ctx, `CREATE EXTENSION IF NOT EXISTS btree_gist`); err != nil {
		return fmt.Errorf("failed to create extension btree_gist: %w", err)
	}
	if _, err := conn.Exec(ctx, ProviderSymbolTable); err != nil {
		return fmt.Errorf("provider_symbol: %w", err)
	}
	if _, err := conn.Exec(ctx, instrumentOHLCView); err != nil {
		return fmt.Errorf("instrument_ohlc: %w", err)
	}
	return nil
}

// ProviderMapping is one half-open [From, To) ticker. A nil To is current.
type ProviderMapping struct {
	InstrumentID string
	Provider     string
	Symbol       string
	From         time.Time
	To           *time.Time
	Evidence     string
}

// UncoveredBarsError is a download bar on a day no rule gives to a company.
// Nothing of that symbol is stored. The text names the ticker and every day.
type UncoveredBarsError struct {
	Symbol string
	Days   []time.Time
}

func (e *UncoveredBarsError) Error() string {
	return fmt.Sprintf("no ticker window covers %s on %s", e.Symbol, formatDayRuns(e.Days))
}

// MappingOverlapError is a new ticker row that still overlaps another
// ticker of the same company after the earlier row was closed at the new
// start, or that cannot be closed because it does not start earlier.
// Nothing of the symbol is stored. The text names both tickers, the company
// and the day.
type MappingOverlapError struct {
	InstrumentID  string
	Symbol, Other string
	Day           time.Time
}

func (e *MappingOverlapError) Error() string {
	return fmt.Sprintf("%s overlaps %s for %s on %s", e.Symbol, e.Other, e.InstrumentID, tradingDay(e.Day))
}

// HeldByError is a pre-window day another company already holds, in
// asset_symbol or in provider_symbol. Nothing of the symbol is stored.
// The text names the ticker, both companies and the day.
type HeldByError struct {
	Symbol      string
	Left, Right string
	Day         time.Time
}

func (e *HeldByError) Error() string {
	return fmt.Sprintf("%s on %s is held by %s and %s", e.Symbol, tradingDay(e.Day), e.Left, e.Right)
}

// ProviderOverlapError is a download in which one provider served two tickers
// of one instrument on the same day. Nothing of that symbol is stored. The
// text names the instrument, the provider, both tickers and the days, and
// it has no next step.
type ProviderOverlapError struct {
	InstrumentID string
	Provider     string
	Left, Right  string
	Days         []time.Time
}

func (e *ProviderOverlapError) Error() string {
	return fmt.Sprintf("%s on %s has prices for both %s and %s on %s",
		e.InstrumentID, priceProvider(e.Provider), e.Left, e.Right, formatDayRuns(e.Days))
}

// IsProviderOverlap reports whether text is that sentence.
func IsProviderOverlap(text string) bool {
	return strings.Contains(text, " has prices for both ")
}

// WriteWindowMapping records one window's mapping on an open transaction.
// The caller commits, so a refused bar in that same transaction leaves no mapping.
func WriteWindowMapping(ctx context.Context, tx pgx.Tx, instrumentID, provider, symbol string, bars []*financepb.OHLC, windowTo *time.Time) error {
	return writeWindowMapping(ctx, tx, instrumentID, provider, symbol, bars, windowTo)
}

// holderWindow is one company's half-open [from, to) hold of a ticker.
type holderWindow struct {
	instrumentID string
	symbol       string
	from         time.Time
	to           *time.Time
}

func (w holderWindow) covers(day time.Time) bool {
	if day.Before(w.from) {
		return false
	}
	return w.to == nil || day.Before(*w.to)
}

// MapDownloadedBars writes one provider_symbol row per company and window
// for the ticker the bars are stored under. A day belongs to the company
// whose asset_symbol window holds that ticker. The company that holds the
// ticker at its first window also keeps days before that window when no
// other company held the ticker, and a bar under the company's latest
// ticker before that window belongs to it on the same condition. A bar no
// rule covers fails the symbol. A stored ticker with no asset_symbol row
// is mapped only through the windows of instrumentID, the company the
// later-ticker lookup resolved, and is left unmapped when that id is empty.
func MapDownloadedBars(ctx context.Context, tx pgx.Tx, instrumentID, asked, provider, storeSymbol string, bars []*financepb.OHLC) error {
	if len(bars) == 0 {
		return nil
	}
	storeSymbol = strings.TrimSpace(storeSymbol)
	asked = strings.TrimSpace(asked)
	if err := duplicateBars(storeSymbol, bars); err != nil {
		return err
	}
	storeWindows, err := loadHolderWindows(ctx, tx, storeSymbol)
	if err != nil {
		return err
	}
	windows := storeWindows
	claimLatest := true
	if len(windows) == 0 {
		if instrumentID == "" {
			return nil
		}
		owned, err := loadInstrumentWindows(ctx, tx, []string{instrumentID})
		if err != nil {
			return err
		}
		for _, w := range owned[instrumentID] {
			if w.symbol == asked {
				windows = append(windows, w)
			}
		}
		if len(windows) == 0 {
			return nil
		}
		claimLatest = false
	}
	aliases := map[string][]holderWindow{}
	if claimLatest {
		ids := make([]string, 0, len(windows))
		seen := map[string]struct{}{}
		for _, w := range windows {
			if _, ok := seen[w.instrumentID]; ok {
				continue
			}
			seen[w.instrumentID] = struct{}{}
			ids = append(ids, w.instrumentID)
		}
		aliases, err = loadInstrumentWindows(ctx, tx, ids)
		if err != nil {
			return err
		}
	}
	type key struct{ id, from, to string }
	groups := map[key]struct {
		window holderWindow
		bars   []*financepb.OHLC
	}{}
	var uncovered []time.Time
	var ambiguous []string
	for _, bar := range bars {
		if bar.GetTimestamp() == nil {
			continue
		}
		day := dateOnly(bar.GetTimestamp().AsTime())
		hits := make([]holderWindow, 0, 1)
		for _, w := range windows {
			if w.covers(day) {
				hits = append(hits, w)
			}
		}
		var chosen *holderWindow
		switch {
		case len(hits) > 1:
			ambiguous = append(ambiguous, fmt.Sprintf("symbol %s maps to %d instruments at %s",
				storeSymbol, len(hits), tradingDay(day)))
		case len(hits) == 1:
			chosen = &hits[0]
		default:
			hold, amb, err := claimBeforeFirstWindow(ctx, tx, provider, storeSymbol, windows, day)
			if err != nil {
				return err
			}
			if amb != "" {
				ambiguous = append(ambiguous, amb)
				continue
			}
			if hold != nil {
				chosen = hold
				break
			}
			if !claimLatest {
				uncovered = append(uncovered, day)
				break
			}
			latestHits := make([]holderWindow, 0, 1)
			for _, idWindows := range aliases {
				if len(idWindows) == 0 {
					continue
				}
				first := idWindows[0].from
				latest := idWindows[len(idWindows)-1]
				if latest.symbol != storeSymbol || day.Before(first) || !day.Before(latest.from) {
					continue
				}
				latestHits = append(latestHits, latest)
			}
			switch {
			case len(latestHits) > 1:
				ambiguous = append(ambiguous, fmt.Sprintf("symbol %s maps to %d instruments at %s",
					storeSymbol, len(latestHits), tradingDay(day)))
			case len(latestHits) == 1:
				chosen = &latestHits[0]
			default:
				uncovered = append(uncovered, day)
			}
		}
		if chosen == nil {
			continue
		}
		to := ""
		if chosen.to != nil {
			to = tradingDay(*chosen.to)
		}
		k := key{chosen.instrumentID, tradingDay(chosen.from), to}
		group := groups[k]
		group.window = *chosen
		group.bars = append(group.bars, bar)
		groups[k] = group
	}
	if len(ambiguous) > 0 {
		sort.Strings(ambiguous)
		return fmt.Errorf("%s: %w", strings.Join(ambiguous, "; "), ErrAmbiguousSymbol)
	}
	if len(uncovered) > 0 {
		return &UncoveredBarsError{Symbol: storeSymbol, Days: uncovered}
	}
	keys := make([]key, 0, len(groups))
	for k := range groups {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].id != keys[j].id {
			return keys[i].id < keys[j].id
		}
		if keys[i].from != keys[j].from {
			return keys[i].from < keys[j].from
		}
		return keys[i].to < keys[j].to
	})
	for _, k := range keys {
		group := groups[k]
		if err := writeWindowMapping(ctx, tx, group.window.instrumentID, provider, storeSymbol, group.bars, group.window.to); err != nil {
			return err
		}
	}
	return nil
}

// claimBeforeFirstWindow is the company that may take day when it falls
// before the earliest window in windows. windows is ordered by from_date.
// More than one company starting that day is the ambiguous-symbol sentence.
// A nil window means the day is not before that first window. Another
// company that already holds the ticker that day, as an alias or as a
// stored mapping, is a HeldByError.
func claimBeforeFirstWindow(ctx context.Context, q postgres.Query, provider, storeSymbol string, windows []holderWindow, day time.Time) (*holderWindow, string, error) {
	if len(windows) == 0 || !day.Before(windows[0].from) {
		return nil, "", nil
	}
	holders := 0
	var hold holderWindow
	for _, w := range windows {
		if !w.from.Equal(windows[0].from) {
			break
		}
		holders++
		hold = w
	}
	if holders > 1 {
		return nil, fmt.Sprintf("symbol %s maps to %d instruments at %s", storeSymbol, holders, tradingDay(day)), nil
	}
	other, ticker, err := otherHolder(ctx, q, provider, hold.symbol, storeSymbol, hold.instrumentID, day)
	if err != nil {
		return nil, "", err
	}
	if other != "" {
		left, right := hold.instrumentID, other
		if right < left {
			left, right = right, left
		}
		return nil, "", &HeldByError{Symbol: ticker, Left: left, Right: right, Day: day}
	}
	return &hold, "", nil
}

// otherHolder is another company that holds ticker on day. An asset_symbol
// window of the asked ticker wins over a provider_symbol row of the stored
// ticker. The returned ticker is the one that company holds.
func otherHolder(ctx context.Context, q postgres.Query, provider, assetSymbol, storeSymbol, claimant string, day time.Time) (string, string, error) {
	var id string
	err := q.QueryRow(ctx, `
		SELECT i.id
		  FROM asset_symbol s
		  JOIN instrument i ON i.isin = s.isin
		 WHERE s.symbol = $1 AND i.id <> $2
		   AND s.from_date <= $3::date
		   AND (s.to_date IS NULL OR $3::date < s.to_date)
		 ORDER BY i.id
		 LIMIT 1`, assetSymbol, claimant, tradingDay(day)).Scan(&id)
	if err == nil {
		return id, assetSymbol, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return "", "", fmt.Errorf("provider_symbol: earlier holder: %w", err)
	}
	err = q.QueryRow(ctx, `
		SELECT instrument_id
		  FROM provider_symbol
		 WHERE provider = $1 AND symbol = $2 AND instrument_id <> $3
		   AND from_date <= $4::date
		   AND (to_date IS NULL OR $4::date < to_date)
		 ORDER BY instrument_id
		 LIMIT 1`, priceProvider(provider), storeSymbol, claimant, tradingDay(day)).Scan(&id)
	if err == nil {
		return id, storeSymbol, nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", nil
	}
	return "", "", fmt.Errorf("provider_symbol: earlier holder: %w", err)
}

func loadHolderWindows(ctx context.Context, q postgres.Query, symbol string) ([]holderWindow, error) {
	rows, err := q.Query(ctx, `
		SELECT i.id, s.symbol, s.from_date, s.to_date
		  FROM asset_symbol s
		  JOIN instrument i ON i.isin = s.isin
		 WHERE s.symbol = $1
		 ORDER BY s.from_date, i.id`, symbol)
	if err != nil {
		return nil, fmt.Errorf("provider_symbol: windows: %w", err)
	}
	defer rows.Close()
	return scanHolderWindows(rows)
}

func loadInstrumentWindows(ctx context.Context, q postgres.Query, instrumentIDs []string) (map[string][]holderWindow, error) {
	rows, err := q.Query(ctx, `
		SELECT i.id, s.symbol, s.from_date, s.to_date
		  FROM instrument i
		  JOIN asset_symbol s ON s.isin = i.isin
		 WHERE i.id = ANY($1::text[])
		 ORDER BY i.id, s.from_date`, instrumentIDs)
	if err != nil {
		return nil, fmt.Errorf("provider_symbol: windows: %w", err)
	}
	defer rows.Close()
	list, err := scanHolderWindows(rows)
	if err != nil {
		return nil, err
	}
	out := map[string][]holderWindow{}
	for _, w := range list {
		out[w.instrumentID] = append(out[w.instrumentID], w)
	}
	return out, nil
}

func scanHolderWindows(rows pgx.Rows) ([]holderWindow, error) {
	out := make([]holderWindow, 0)
	for rows.Next() {
		var w holderWindow
		var to *time.Time
		if err := rows.Scan(&w.instrumentID, &w.symbol, &w.from, &to); err != nil {
			return nil, fmt.Errorf("provider_symbol: windows: %w", err)
		}
		w.from = dateOnly(w.from)
		if to != nil {
			end := dateOnly(*to)
			w.to = &end
		}
		out = append(out, w)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("provider_symbol: windows: %w", err)
	}
	return out, nil
}

// writeWindowMapping records symbol from the first bar through the window
// end. A nil end is open. A later download that starts earlier moves the
// same row's from_date back and does not open the end. An earlier row of
// another ticker that runs into this span is closed at the new start. A
// span that still overlaps fails with MappingOverlapError and is not written.
func writeWindowMapping(ctx context.Context, tx pgx.Tx, instrumentID, provider, symbol string, bars []*financepb.OHLC, windowTo *time.Time) error {
	days := barDaysOf(bars)
	if len(days) == 0 {
		return nil
	}
	provider = priceProvider(provider)
	symbol = strings.TrimSpace(symbol)
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext('provider_symbol:' || $1 || ':' || $2))`,
		instrumentID, provider); err != nil {
		return fmt.Errorf("provider_symbol: %w", err)
	}
	existing, err := listProviderMappings(ctx, tx, instrumentID, provider)
	if err != nil {
		return err
	}
	first := days[0]
	for _, row := range existing {
		if row.Symbol == symbol {
			continue
		}
		shared, err := sharedBarDays(ctx, tx, provider, row.Symbol, days)
		if err != nil {
			return err
		}
		if len(shared) > 0 {
			return &ProviderOverlapError{
				InstrumentID: instrumentID, Provider: provider,
				Left: row.Symbol, Right: symbol, Days: shared,
			}
		}
	}
	for i := range existing {
		row := &existing[i]
		if row.Symbol == symbol || !row.From.Before(first) {
			continue
		}
		if row.To != nil && !row.To.After(first) {
			continue
		}
		if _, err := tx.Exec(ctx, `
			UPDATE provider_symbol SET to_date = $4::date
			 WHERE instrument_id = $1 AND provider = $2 AND from_date = $3::date`,
			instrumentID, provider, tradingDay(row.From), tradingDay(first)); err != nil {
			return fmt.Errorf("provider_symbol: close %s: %w", row.Symbol, err)
		}
		closed := first
		row.To = &closed
	}
	for _, row := range existing {
		if row.Symbol == symbol {
			continue
		}
		if rangesOverlap(first, windowTo, row.From, row.To) {
			return &MappingOverlapError{
				InstrumentID: instrumentID, Symbol: symbol, Other: row.Symbol, Day: first,
			}
		}
	}
	for _, row := range existing {
		if row.Symbol != symbol || !windowEndCovers(row.To, windowTo) {
			continue
		}
		if !row.From.After(first) {
			return nil
		}
		if _, err := tx.Exec(ctx, `
			UPDATE provider_symbol SET from_date = $4::date
			 WHERE instrument_id = $1 AND provider = $2 AND from_date = $3::date`,
			instrumentID, provider, tradingDay(row.From), tradingDay(first)); err != nil {
			return fmt.Errorf("provider_symbol: extend %s: %w", symbol, err)
		}
		return nil
	}
	var toArg any
	if windowTo != nil {
		toArg = tradingDay(*windowTo)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO provider_symbol (instrument_id, provider, symbol, from_date, to_date, evidence)
		VALUES ($1, $2, $3, $4::date, $5::date, 'download')`,
		instrumentID, provider, symbol, tradingDay(first), toArg); err != nil {
		return fmt.Errorf("provider_symbol: store %s: %w", symbol, err)
	}
	return nil
}

// rangesOverlap reports whether the half-open spans [aFrom, aTo) and
// [bFrom, bTo) share a day. A nil end is open.
func rangesOverlap(aFrom time.Time, aTo *time.Time, bFrom time.Time, bTo *time.Time) bool {
	if aTo != nil && !aTo.After(bFrom) {
		return false
	}
	if bTo != nil && !bTo.After(aFrom) {
		return false
	}
	return true
}

// windowEndCovers reports that have ends at or after need. A nil end is open.
func windowEndCovers(have, need *time.Time) bool {
	if have == nil {
		return true
	}
	if need == nil {
		return false
	}
	return !have.Before(*need)
}

func listProviderMappings(ctx context.Context, q postgres.Query, instrumentID, provider string) ([]ProviderMapping, error) {
	rows, err := q.Query(ctx, `
		SELECT symbol, from_date, to_date, evidence
		  FROM provider_symbol
		 WHERE instrument_id = $1 AND provider = $2
		 ORDER BY from_date`, instrumentID, provider)
	if err != nil {
		return nil, fmt.Errorf("provider_symbol: list: %w", err)
	}
	defer rows.Close()
	out := make([]ProviderMapping, 0)
	for rows.Next() {
		var row ProviderMapping
		var to *time.Time
		if err := rows.Scan(&row.Symbol, &row.From, &to, &row.Evidence); err != nil {
			return nil, fmt.Errorf("provider_symbol: list: %w", err)
		}
		row.InstrumentID = instrumentID
		row.Provider = provider
		row.From = dateOnly(row.From)
		if to != nil {
			t := dateOnly(*to)
			row.To = &t
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("provider_symbol: list: %w", err)
	}
	return out, nil
}

// sharedBarDays reports which of days already have a stored bar under
// symbol. days is only the bars in this download. A stored row on any
// other day, including a row under the other ticker on a day this
// download does not include, is not looked at and does not refuse it.
func sharedBarDays(ctx context.Context, q postgres.Query, provider, symbol string, days []time.Time) ([]time.Time, error) {
	texts := make([]string, len(days))
	for i, day := range days {
		texts[i] = tradingDay(day)
	}
	rows, err := q.Query(ctx, `
		SELECT DISTINCT (o.time AT TIME ZONE 'UTC')::date
		  FROM ohlc o
		 WHERE o.provider = $1 AND o.symbol = $2
		   AND (o.time AT TIME ZONE 'UTC')::date = ANY($3::date[])
		 ORDER BY 1`, provider, symbol, texts)
	if err != nil {
		return nil, fmt.Errorf("provider_symbol: shared days: %w", err)
	}
	defer rows.Close()
	shared := make([]time.Time, 0)
	for rows.Next() {
		var day time.Time
		if err := rows.Scan(&day); err != nil {
			return nil, fmt.Errorf("provider_symbol: shared days: %w", err)
		}
		shared = append(shared, dateOnly(day))
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("provider_symbol: shared days: %w", err)
	}
	return shared, nil
}

func duplicateBars(symbol string, bars []*financepb.OHLC) error {
	seen := make(map[string]struct{}, len(bars))
	for _, bar := range bars {
		if bar.GetTimestamp() == nil {
			continue
		}
		day := bar.GetTimestamp().AsTime()
		key := tradingDay(day)
		if _, ok := seen[key]; ok {
			return &DuplicateDayError{Symbol: symbol, Day: day}
		}
		seen[key] = struct{}{}
	}
	return nil
}

func barDaysOf(bars []*financepb.OHLC) []time.Time {
	seen := make(map[string]time.Time, len(bars))
	for _, bar := range bars {
		if bar.GetTimestamp() == nil {
			continue
		}
		day := dateOnly(bar.GetTimestamp().AsTime())
		seen[tradingDay(day)] = day
	}
	out := make([]time.Time, 0, len(seen))
	for _, day := range seen {
		out = append(out, day)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Before(out[j]) })
	return out
}

func formatDayRuns(days []time.Time) string {
	if len(days) == 0 {
		return ""
	}
	ordered := append([]time.Time(nil), days...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].Before(ordered[j]) })
	parts := make([]string, 0)
	start, prev := ordered[0], ordered[0]
	flush := func() {
		if start.Equal(prev) {
			parts = append(parts, tradingDay(start))
			return
		}
		parts = append(parts, tradingDay(start)+" to "+tradingDay(prev))
	}
	for _, day := range ordered[1:] {
		if day.Equal(prev.AddDate(0, 0, 1)) {
			prev = day
			continue
		}
		flush()
		start, prev = day, day
	}
	flush()
	return strings.Join(parts, ", ")
}

// InstrumentBars reads one instrument's mapped bars on [start, end], both
// days included, oldest first. A session the mapping does not cover is absent.
func (db *OHLC) InstrumentBars(ctx context.Context, provider, instrumentID string, start, end time.Time) ([]*financepb.OHLC, error) {
	if strings.TrimSpace(instrumentID) == "" {
		return nil, errors.New("instrument id is empty")
	}
	provider = priceProvider(provider)
	spanStart, _ := dayBounds(start)
	_, spanEnd := dayBounds(end)
	rows, err := db.Conn.Query(ctx, `
		SELECT symbol, time, open::float8, high::float8, low::float8, close::float8, volume
		  FROM instrument_ohlc
		 WHERE provider = $1 AND instrument_id = $2
		   AND time >= $3 AND time < $4
		 ORDER BY time`, provider, instrumentID, spanStart, spanEnd)
	if err != nil {
		return nil, fmt.Errorf("failed to read instrument bars: %w", err)
	}
	defer rows.Close()
	out := make([]*financepb.OHLC, 0)
	for rows.Next() {
		var (
			symbol     string
			at         time.Time
			o, h, l, c float64
			volume     int64
		)
		if err := rows.Scan(&symbol, &at, &o, &h, &l, &c, &volume); err != nil {
			return nil, fmt.Errorf("failed to read instrument bars: %w", err)
		}
		if volume < 0 {
			return nil, fmt.Errorf("ohlc volume negative for %s", symbol)
		}
		out = append(out, &financepb.OHLC{
			Symbol: symbol, Timestamp: timestamppb.New(at.UTC()),
			Open: o, High: h, Low: l, Close: c, Volume: uint64(volume),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to read instrument bars: %w", err)
	}
	return out, nil
}

type tickerSpan struct {
	symbol      string
	first, last time.Time
	to          *time.Time
}

// BackfillProviderSymbols writes provider_symbol from asset_symbol tickers
// that already have bars, and marks request-dated aliases. It deletes
// nothing. An instrument that already has a mapping is left as it is. A
// day both of a company's tickers have a bar is left unmapped and named
// in one error log; the broker still starts. A second call writes nothing
// new.
func BackfillProviderSymbols(ctx context.Context, conn postgres.Query) error {
	if err := markRequestDatedAliases(ctx, conn); err != nil {
		return err
	}
	rows, err := conn.Query(ctx, `
		SELECT i.id, o.provider, s.symbol,
		       min((o.time AT TIME ZONE 'UTC')::date),
		       max((o.time AT TIME ZONE 'UTC')::date),
		       s.to_date
		  FROM asset_symbol s
		  JOIN instrument i ON i.isin = s.isin
		  JOIN ohlc o ON o.symbol = s.symbol
		   AND (o.time AT TIME ZONE 'UTC')::date >= s.from_date
		   AND (s.to_date IS NULL OR (o.time AT TIME ZONE 'UTC')::date < s.to_date)
		 GROUP BY i.id, o.provider, s.symbol, s.from_date, s.to_date`)
	if err != nil {
		return fmt.Errorf("provider_symbol backfill: %w", err)
	}
	defer rows.Close()
	type key struct{ id, provider string }
	groups := make(map[key][]tickerSpan)
	for rows.Next() {
		var id, provider, symbol string
		var first, last time.Time
		var closed *time.Time
		if err := rows.Scan(&id, &provider, &symbol, &first, &last, &closed); err != nil {
			return fmt.Errorf("provider_symbol backfill: %w", err)
		}
		k := key{id, priceProvider(provider)}
		span := tickerSpan{symbol: symbol, first: dateOnly(first), last: dateOnly(last)}
		if closed != nil {
			end := dateOnly(*closed)
			span.to = &end
		}
		groups[k] = append(groups[k], span)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("provider_symbol backfill: %w", err)
	}
	byInstrument := make(map[string]map[string][]tickerSpan)
	for k, spans := range groups {
		providers := byInstrument[k.id]
		if providers == nil {
			providers = make(map[string][]tickerSpan)
			byInstrument[k.id] = providers
		}
		providers[k.provider] = append(providers[k.provider], spans...)
	}
	ids := make([]string, 0, len(byInstrument))
	for id := range byInstrument {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	var skipped []string
	for _, id := range ids {
		lines, err := backfillInstrument(ctx, conn, id, byInstrument[id])
		if err != nil {
			return err
		}
		skipped = append(skipped, lines...)
	}
	if len(skipped) > 0 {
		sort.Strings(skipped)
		log.Error().Strs("overlaps", skipped).
			Msg("provider_symbol backfill left instruments unmapped because their tickers overlap: " + strings.Join(skipped, "; "))
	}
	return nil
}

// backfillInstrument writes one instrument's mappings in one transaction.
// A constraint conflict rolls that instrument back, logs it, and leaves the
// instrument unmapped. Startup still returns nil.
func backfillInstrument(ctx context.Context, conn postgres.Query, instrumentID string, providers map[string][]tickerSpan) ([]string, error) {
	begin, ok := conn.(txBeginner)
	if !ok {
		return nil, fmt.Errorf("provider_symbol backfill: connection can't open a transaction")
	}
	tx, err := begin.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("provider_symbol backfill: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op after Commit

	names := make([]string, 0, len(providers))
	for provider := range providers {
		names = append(names, provider)
	}
	sort.Strings(names)
	var skipped []string
	for _, provider := range names {
		line, err := backfillOne(ctx, tx, instrumentID, provider, providers[provider])
		if isMappingConflict(err) {
			log.Error().Err(err).Str("instrument", instrumentID).Str("provider", provider).
				Msg("provider_symbol backfill left an instrument unmapped because the mapping conflicted")
			return nil, nil
		}
		if err != nil {
			return nil, err
		}
		if line != "" {
			skipped = append(skipped, line)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		if isMappingConflict(err) {
			log.Error().Err(err).Str("instrument", instrumentID).
				Msg("provider_symbol backfill left an instrument unmapped because the mapping conflicted")
			return nil, nil
		}
		return nil, fmt.Errorf("provider_symbol backfill: %w", err)
	}
	return skipped, nil
}

func isMappingConflict(err error) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}
	// 23P01 exclusion violation, 23505 unique violation.
	return pgErr.Code == "23P01" || pgErr.Code == "23505"
}

func backfillOne(ctx context.Context, conn postgres.Query, instrumentID, provider string, spans []tickerSpan) (string, error) {
	sort.Slice(spans, func(i, j int) bool {
		if spans[i].first.Equal(spans[j].first) {
			return spans[i].symbol < spans[j].symbol
		}
		return spans[i].first.Before(spans[j].first)
	})
	// A download can already have mapped this instrument. Yahoo stores the
	// whole history under META, so those bars sit inside FB's window, and
	// checking them again on every start would log a false overlap.
	existing, err := listProviderMappings(ctx, conn, instrumentID, provider)
	if err != nil {
		return "", err
	}
	if len(existing) > 0 {
		return "", nil
	}
	// Own windows never overlap, so a bar span inside each window cannot
	// either. An overlap is a day both tickers have a bar. A META bar inside
	// FB's window with no FB bar that day is one series, not two prices.
	line, err := crossWindowLine(ctx, conn, instrumentID, provider)
	if err != nil {
		return "", err
	}
	if line != "" {
		return line, nil
	}
	for i, span := range spans {
		var to any
		switch {
		case span.to != nil:
			to = tradingDay(*span.to)
		case i+1 < len(spans):
			to = tradingDay(spans[i+1].first)
		}
		if _, err := conn.Exec(ctx, `
			INSERT INTO provider_symbol (instrument_id, provider, symbol, from_date, to_date, evidence)
			VALUES ($1, $2, $3, $4::date, $5::date, 'migration')`,
			instrumentID, provider, span.symbol, tradingDay(span.first), to); err != nil {
			return "", fmt.Errorf("provider_symbol backfill %s %s: %w", instrumentID, provider, err)
		}
	}
	return "", nil
}

// crossWindowLine names days both of this company's tickers have a bar
// and one of those bars falls inside the other ticker's window. A day
// another company held the stored ticker is that company's bar, not this
// overlap. An empty string means the instrument can be mapped.
func crossWindowLine(ctx context.Context, conn postgres.Query, instrumentID, provider string) (string, error) {
	rows, err := conn.Query(ctx, `
		SELECT holder.symbol, other.symbol, (o.time AT TIME ZONE 'UTC')::date
		  FROM instrument i
		  JOIN asset_symbol holder ON holder.isin = i.isin
		  JOIN asset_symbol other
		    ON other.isin = holder.isin AND other.symbol <> holder.symbol
		  JOIN ohlc o ON o.provider = $2 AND o.symbol = holder.symbol
		   AND (o.time AT TIME ZONE 'UTC')::date >= other.from_date
		   AND (other.to_date IS NULL OR (o.time AT TIME ZONE 'UTC')::date < other.to_date)
		 WHERE i.id = $1
		   AND EXISTS (
				SELECT 1
				  FROM ohlc also
				 WHERE also.provider = o.provider
				   AND also.symbol = other.symbol
				   AND (also.time AT TIME ZONE 'UTC')::date = (o.time AT TIME ZONE 'UTC')::date
		   )
		   AND NOT EXISTS (
				SELECT 1
				  FROM asset_symbol foreign_hold
				  JOIN instrument foreign_inst ON foreign_inst.isin = foreign_hold.isin
				 WHERE foreign_inst.id <> i.id
				   AND foreign_hold.symbol = holder.symbol
				   AND (o.time AT TIME ZONE 'UTC')::date >= foreign_hold.from_date
				   AND (foreign_hold.to_date IS NULL
				        OR (o.time AT TIME ZONE 'UTC')::date < foreign_hold.to_date)
		   )`, instrumentID, provider)
	if err != nil {
		return "", fmt.Errorf("provider_symbol backfill: %w", err)
	}
	defer rows.Close()
	type pair struct{ left, right string }
	days := make(map[pair]map[string]struct{})
	for rows.Next() {
		var stored, window string
		var day time.Time
		if err := rows.Scan(&stored, &window, &day); err != nil {
			return "", fmt.Errorf("provider_symbol backfill: %w", err)
		}
		left, right := stored, window
		if right < left {
			left, right = right, left
		}
		key := pair{left, right}
		if days[key] == nil {
			days[key] = make(map[string]struct{})
		}
		days[key][tradingDay(dateOnly(day))] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return "", fmt.Errorf("provider_symbol backfill: %w", err)
	}
	if len(days) == 0 {
		return "", nil
	}
	parts := make([]string, 0, len(days))
	for key, set := range days {
		listed := make([]string, 0, len(set))
		for day := range set {
			listed = append(listed, day)
		}
		sort.Strings(listed)
		parts = append(parts, fmt.Sprintf("%s and %s on %s", key.left, key.right, strings.Join(listed, ", ")))
	}
	sort.Strings(parts)
	return fmt.Sprintf("%s %s %s", instrumentID, provider, strings.Join(parts, "; ")), nil
}

// markRequestDatedAliases sets evidence later_ticker on an alias the download
// closed onto the day after a backtest end, when that day is not a month-end
// snapshot as-of. A month-end can be a snapshot, so it stays snapshot.
func markRequestDatedAliases(ctx context.Context, conn postgres.Query) error {
	var periods bool
	if err := conn.QueryRow(ctx, `SELECT to_regclass('backtest_period') IS NOT NULL`).Scan(&periods); err != nil {
		return fmt.Errorf("asset_symbol evidence: %w", err)
	}
	if !periods {
		return nil
	}
	_, err := conn.Exec(ctx, `
		UPDATE asset_symbol s
		   SET evidence = 'later_ticker'
		 WHERE s.evidence = 'snapshot'
		   AND EXISTS (
			SELECT 1 FROM asset_symbol prev
			 WHERE prev.isin = s.isin
			   AND prev.symbol <> s.symbol
			   AND prev.to_date = s.from_date)
		   AND EXISTS (
			SELECT 1 FROM backtest_period p
			 WHERE p.end_date + 1 = s.from_date)
		   AND s.from_date <> (date_trunc('month', s.from_date) + interval '1 month - 1 day')::date`)
	if err != nil {
		return fmt.Errorf("asset_symbol evidence: %w", err)
	}
	return nil
}
