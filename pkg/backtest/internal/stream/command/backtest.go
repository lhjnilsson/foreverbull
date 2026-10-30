package command

import (
	"context"
	"errors"
	"fmt"
	"io"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/internal/storage"
	"github.com/lhjnilsson/foreverbull/internal/stream"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/engine"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/repository"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/stream/dependency"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/universe"
	ss "github.com/lhjnilsson/foreverbull/pkg/backtest/stream"
	"github.com/lhjnilsson/foreverbull/pkg/finance"
	"github.com/lhjnilsson/foreverbull/pkg/finance/supplier"
	"github.com/lhjnilsson/foreverbull/pkg/pb"
	"github.com/lhjnilsson/foreverbull/pkg/pb/backtest"
	financeServicePB "github.com/lhjnilsson/foreverbull/pkg/pb/finance/service"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

func CreateBacktestFromBacktest(ctx context.Context, msg stream.Message) error {
	command := ss.CreateBacktestFromBacktestCommand{}
	err := msg.ParsePayload(&command)
	if err != nil {
		return fmt.Errorf("error unmarshalling CreateBacktest payload: %w", err)
	}

	b := backtest.Backtest{}
	err = proto.Unmarshal(command.Backtest, &b)
	if err != nil {
		return fmt.Errorf("error unmarshalling backtest: %w", err)
	}

	// A universe file may spell a share class BRK.B, BRK-B or BRKB; all are
	// stored as BRKB. ETF holdings keep their own names (aliases).
	if err = storeShareClassesOnce(&b); err != nil {
		return fmt.Errorf("error creating backtest: %w", err)
	}

	db := msg.MustGet(stream.DBDep).(*pgxpool.Pool)
	backtests := repository.Backtest{Conn: db}
	if err = createStoredBacktest(ctx, backtests, command.OwnerID, &b); err != nil {
		return fmt.Errorf("error creating backtest: %w", err)
	}
	return nil
}

func CreateBacktestFromETF(ctx context.Context, msg stream.Message) error {
	command := ss.CreateBacktestFromETFCommand{}

	err := msg.ParsePayload(&command)
	if err != nil {
		return fmt.Errorf("error unmarshalling CreateBacktest payload: %w", err)
	}
	command.BacktestName = strings.TrimSpace(command.BacktestName)
	command.ETFName = strings.TrimSpace(command.ETFName)
	command.StartDate = strings.TrimSpace(command.StartDate)
	command.EndDate = strings.TrimSpace(command.EndDate)

	// Same sentences as the create prompt. Fail before any holdings download.
	start, end, err := validateETFCreateInput(command.BacktestName, command.ETFName, command.StartDate, command.EndDate, time.Now())
	if err != nil {
		return err
	}

	etfscraper, isETFScraper := msg.MustGet(dependency.GetETFScraper).(dependency.ETFScraper)
	if !isETFScraper {
		return errors.New("error casting ETFScraper")
	}

	snapshots, err := loadETFSnapshots(etfscraper, command.ETFName, start, end)
	if err != nil {
		return err
	}
	result := universe.Diff(snapshots)

	benchmark, err := etfscraper.GetBenchmark(command.ETFName)
	if err != nil {
		return fmt.Errorf("error getting benchmark: %w", err)
	}

	marketdataClient, isMarketdataClient := msg.MustGet(dependency.GetMarketdataClient).(financeServicePB.MarketdataClient)
	if !isMarketdataClient {
		return errors.New("error casting marketdata client")
	}

	symbols := uniqueAliasSymbols(result.Aliases)
	hasData := vendorHasData(ctx, marketdataClient, symbols)

	b := backtest.Backtest{
		Name:      command.BacktestName,
		Benchmark: benchmark,
		Provider:  command.Provider,
		Profile:   command.Profile,
		Periods:   calendarYearPeriods(start, end),
	}
	fillPeriodAssets(b.Periods, result.Aliases, hasData)

	if err := requireStoredOwner(command.OwnerID, ""); err != nil {
		return err
	}
	db := msg.MustGet(stream.DBDep).(*pgxpool.Pool)
	backtests := repository.Backtest{Conn: db}
	if err = createStoredBacktest(ctx, backtests, command.OwnerID, &b); err != nil {
		return fmt.Errorf("error creating backtest: %w", err)
	}
	backtestID := ""
	if environment.GetAuthEnabled() {
		backtestID, err = backtests.IDByOwnerAndName(ctx, command.OwnerID, command.BacktestName)
		if err != nil {
			return fmt.Errorf("error getting backtest: %w", err)
		}
	}
	if err := persistUniverse(ctx, db, command.BacktestName, backtestID, snapshots, result); err != nil {
		return err
	}

	return nil
}

var isoDatePattern = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

func validateETFCreateInput(name, etf, startRaw, endRaw string, now time.Time) (time.Time, time.Time, error) {
	name = strings.TrimSpace(name)
	etf = strings.TrimSpace(etf)
	startRaw = strings.TrimSpace(startRaw)
	endRaw = strings.TrimSpace(endRaw)
	if name == "" {
		return time.Time{}, time.Time{}, plainSentence("Enter a name for the backtest.")
	}
	product, ok := etfByName(etf)
	if !ok {
		return time.Time{}, time.Time{}, plainSentence("Unknown ETF " + pythonRepr(etf) + ". Known funds: " + knownETFNames() + ".")
	}
	start, ok := parseISODate(startRaw)
	if !ok {
		return time.Time{}, time.Time{}, plainSentence(pythonRepr(startRaw) + " is not a date. Use YYYY-MM-DD, for example 2024-01-31.")
	}
	if start.Before(product.Inception) {
		return time.Time{}, time.Time{}, plainSentence(etf + " started on " + product.Inception.Format("2006-01-02") + ". Choose a start date on or after that day.")
	}
	today := dateOnly(now)
	if start.After(today) {
		return time.Time{}, time.Time{}, plainSentence(start.Format("2006-01-02") + " is after today (" + today.Format("2006-01-02") + ").")
	}
	end, ok := parseISODate(endRaw)
	if !ok {
		return time.Time{}, time.Time{}, plainSentence(pythonRepr(endRaw) + " is not a date. Use YYYY-MM-DD, for example 2024-01-31.")
	}
	if end.Before(start) {
		return time.Time{}, time.Time{}, plainSentence("End date " + end.Format("2006-01-02") + " is before start date " + start.Format("2006-01-02") + ".")
	}
	if end.After(today) {
		return time.Time{}, time.Time{}, plainSentence("End date " + end.Format("2006-01-02") + " is after today (" + today.Format("2006-01-02") + ").")
	}
	return start, end, nil
}

// plainSentence is the prompt text. It is not wrapped, so callers see that sentence only.
func plainSentence(text string) error {
	return errors.New(text)
}

func etfByName(name string) (dependency.ETFProduct, bool) {
	for _, product := range dependency.ETFCatalog() {
		if product.Name == name {
			return product, true
		}
	}
	return dependency.ETFProduct{}, false
}

func knownETFNames() string {
	products := dependency.ETFCatalog()
	names := make([]string, len(products))
	for i, product := range products {
		names[i] = product.Name
	}
	return strings.Join(names, ", ")
}

func parseISODate(raw string) (time.Time, bool) {
	if !isoDatePattern.MatchString(raw) {
		return time.Time{}, false
	}
	parsed, err := time.Parse("2006-01-02", raw)
	if err != nil {
		return time.Time{}, false
	}
	parsed = dateOnly(parsed)
	if parsed.Format("2006-01-02") != raw {
		return time.Time{}, false
	}
	return parsed, true
}

// pythonRepr matches Python str repr so the server sentence is the prompt sentence.
func pythonRepr(s string) string {
	quote := "'"
	if strings.Contains(s, "'") && !strings.Contains(s, `"`) {
		quote = `"`
	}
	var b strings.Builder
	b.WriteString(quote)
	for _, r := range s {
		switch r {
		case '\\':
			b.WriteString(`\\`)
		case '\'':
			if quote == "'" {
				b.WriteString(`\'`)
			} else {
				b.WriteRune(r)
			}
		case '"':
			if quote == `"` {
				b.WriteString(`\"`)
			} else {
				b.WriteRune(r)
			}
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			if r < 0x20 {
				fmt.Fprintf(&b, `\x%02x`, r)
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteString(quote)
	return b.String()
}

func loadETFSnapshots(scraper dependency.ETFScraper, etf string, start, end time.Time) ([]universe.Snapshot, error) {
	listings, err := dependency.LoadSnapshotWindow(scraper, etf, start, end)
	if err != nil {
		return nil, err
	}
	listings = seedBacktestStart(etf, listings, start)
	recordLeadingHoldingsGap(etf, listings)

	snaps := make([]universe.Snapshot, 0, len(listings))
	for _, listing := range listings {
		holdings := holdingsFromAssets(listing.Assets)
		if len(holdings) == 0 {
			continue
		}
		// Copied rows keep the source ISIN and ticker, so a gap is not a drop or a rename.
		snaps = append(snaps, universe.Snapshot{AsOf: listing.AsOf, Holdings: holdings})
	}
	if len(snaps) == 0 {
		return nil, fmt.Errorf("iShares has no holdings file for %s between %s and %s",
			etf, dateOnly(start).Format("2006-01-02"), dateOnly(end).Format("2006-01-02"))
	}
	return snaps, nil
}

// seedBacktestStart opens membership on the backtest start from the last list
// published on or before it. Diff opens every interval at the first as-of, so
// that list is moved to the start and held until the next as-of. Listings
// before the start are dropped; the backtest does not read them.
//
// A list published after the start is a look-ahead. It is used only when
// nothing was published on or before the start, so the universe is not empty,
// and the warning names the ETF, the start, and the as-of that was used.
func seedBacktestStart(etf string, listings []dependency.Listing, start time.Time) []dependency.Listing {
	start = dateOnly(start)
	src := 0
	for i := range listings {
		if !dateOnly(listings[i].AsOf).After(start) {
			src = i
		}
	}
	published := publishedAsOf(listings[src])
	if published.After(start) {
		log.Warn().
			Str("etf", etf).
			Str("start_date", start.Format("2006-01-02")).
			Str("snapshot_date", published.Format("2006-01-02")).
			Msg("no holdings snapshot published on or before the backtest start, seeded from the first one after it")
	}
	seeded := make([]dependency.Listing, 0, len(listings)+1)
	seeded = append(seeded, dependency.Listing{
		AsOf:       start,
		Assets:     slices.Clone(listings[src].Assets),
		FilledFrom: published,
	})
	for _, listing := range listings {
		if dateOnly(listing.AsOf).After(start) {
			seeded = append(seeded, listing)
		}
	}
	return seeded
}

// publishedAsOf is the as-of of the file a listing's rows came from.
func publishedAsOf(listing dependency.Listing) time.Time {
	if listing.FilledFrom.IsZero() {
		return dateOnly(listing.AsOf)
	}
	return dateOnly(listing.FilledFrom)
}

// recordLeadingHoldingsGap logs that a leading gap kept a later published listing.
func recordLeadingHoldingsGap(etf string, listings []dependency.Listing) {
	var first, last, from time.Time
	count := 0
	for _, listing := range listings {
		if !listing.Leading || listing.FilledFrom.IsZero() {
			continue
		}
		count++
		if count == 1 || listing.AsOf.Before(first) {
			first = listing.AsOf
			from = listing.FilledFrom
		}
		if !listing.AsOf.Before(last) {
			last = listing.AsOf
		}
	}
	if count == 0 {
		return
	}
	log.Warn().
		Str("etf", etf).
		Str("gap_start", first.Format("2006-01-02")).
		Str("gap_end", last.Format("2006-01-02")).
		Str("filled_from", from.Format("2006-01-02")).
		Msg("leading holdings gap filled from a later published listing")
}

func holdingsFromAssets(assets []dependency.Asset) []universe.Holding {
	holdings := make([]universe.Holding, 0, len(assets))
	for _, asset := range assets {
		holdings = append(holdings, universe.Holding{
			ISIN:      asset.ISIN,
			Symbol:    asset.Symbol,
			Name:      asset.Name,
			AssetType: asset.AssetType,
		})
	}
	return holdings
}

func calendarYearPeriods(start, end time.Time) []*backtest.Backtest_Period {
	periods := make([]*backtest.Backtest_Period, 0, end.Year()-start.Year()+1)
	for year := start.Year(); year <= end.Year(); year++ {
		periods = append(periods, &backtest.Backtest_Period{
			StartDate: &pb.Date{Year: int32(year), Month: 1, Day: 1},
			EndDate:   &pb.Date{Year: int32(year), Month: 12, Day: 31},
		})
	}
	return periods
}

func uniqueAliasSymbols(aliases []universe.Alias) []string {
	seen := make(map[string]struct{}, len(aliases))
	symbols := make([]string, 0, len(aliases))
	for _, alias := range aliases {
		if alias.Symbol == "" {
			continue
		}
		if _, ok := seen[alias.Symbol]; ok {
			continue
		}
		seen[alias.Symbol] = struct{}{}
		symbols = append(symbols, alias.Symbol)
	}
	sort.Strings(symbols)
	return symbols
}

func vendorHasData(ctx context.Context, client financeServicePB.MarketdataClient, symbols []string) map[string]bool {
	hasData := make(map[string]bool, len(symbols))
	for _, symbol := range symbols {
		_, err := client.GetAsset(ctx, &financeServicePB.GetAssetRequest{
			Identifier: &financeServicePB.GetAssetRequest_Symbol{Symbol: symbol},
		})
		if err != nil {
			log.Warn().Err(err).Str("symbol", symbol).Msg("error getting asset, marking as no data")
			hasData[symbol] = false
			continue
		}
		hasData[symbol] = true
	}
	return hasData
}

func fillPeriodAssets(periods []*backtest.Backtest_Period, aliases []universe.Alias, hasData map[string]bool) {
	for _, period := range periods {
		start := pb.DateToTime(period.StartDate)
		end := pb.DateToTime(period.EndDate)
		seen := make(map[string]struct{})
		assets := make([]*backtest.Backtest_Period_Asset, 0)
		for _, alias := range aliases {
			if alias.Symbol == "" || !intervalOverlapsPeriod(alias.FromDate, alias.ToDate, start, end) {
				continue
			}
			if _, ok := seen[alias.Symbol]; ok {
				continue
			}
			seen[alias.Symbol] = struct{}{}
			assets = append(assets, &backtest.Backtest_Period_Asset{
				Symbol:  alias.Symbol,
				Enabled: hasData[alias.Symbol],
			})
		}
		sort.Slice(assets, func(i, j int) bool { return assets[i].Symbol < assets[j].Symbol })
		period.Assets = assets
	}
}

func intervalOverlapsPeriod(from time.Time, to *time.Time, start, end time.Time) bool {
	from = dateOnly(from)
	start = dateOnly(start)
	end = dateOnly(end)
	if from.After(end) {
		return false
	}
	if to == nil {
		return true
	}
	return dateOnly(*to).After(start)
}

func persistUniverse(ctx context.Context, db *pgxpool.Pool, backtestName, backtestID string, snaps []universe.Snapshot, result universe.Result) error {
	names := instrumentNames(snaps)
	instruments := repository.Instrument{Conn: db}
	for _, member := range result.Members {
		if err := instruments.Upsert(ctx, member.ISIN, names[member.ISIN]); err != nil {
			return fmt.Errorf("error storing instrument %s: %w", member.ISIN, err)
		}
	}
	for _, alias := range result.Aliases {
		if err := instruments.StoreAlias(ctx, repository.AliasInterval{
			ISIN:     alias.ISIN,
			Symbol:   alias.Symbol,
			FromDate: alias.FromDate,
			ToDate:   alias.ToDate,
		}); err != nil {
			return fmt.Errorf("error recording symbol %s for %s: %w", alias.Symbol, alias.ISIN, err)
		}
	}

	intervals := make([]repository.MemberInterval, 0, len(result.Members))
	for _, member := range result.Members {
		intervals = append(intervals, repository.MemberInterval{
			ISIN:     member.ISIN,
			FromDate: member.FromDate,
			ToDate:   member.ToDate,
		})
	}
	members := repository.UniverseMember{Conn: db}
	if environment.GetAuthEnabled() {
		if backtestID == "" {
			return errors.New("backtest id is empty")
		}
		if err := members.ReplaceByID(ctx, backtestID, intervals); err != nil {
			return fmt.Errorf("error storing universe members: %w", err)
		}
		return nil
	}
	if err := members.Replace(ctx, backtestName, intervals); err != nil {
		return fmt.Errorf("error storing universe members: %w", err)
	}
	return nil
}

func instrumentNames(snaps []universe.Snapshot) map[string]string {
	names := make(map[string]string)
	for _, snap := range snaps {
		for _, holding := range snap.Holdings {
			if holding.ISIN == "" {
				continue
			}
			names[holding.ISIN] = holding.Name
		}
	}
	return names
}

func dateOnly(t time.Time) time.Time {
	year, month, day := t.Date()
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}

func ingestSymbols(ctx context.Context, db *pgxpool.Pool, b *backtest.Backtest, backtestID string) ([]string, error) {
	symbols := []string{b.Benchmark}
	column := "m.backtest_name"
	key := b.Name
	if environment.GetAuthEnabled() {
		if backtestID == "" {
			return nil, errors.New("backtest id is empty")
		}
		column = "m.backtest_id"
		key = backtestID
	}

	rows, err := db.Query(ctx, `
		SELECT DISTINCT s.symbol
		  FROM universe_member m
		  JOIN asset_symbol s ON s.isin = m.isin
		 WHERE `+column+` = $1
		 ORDER BY s.symbol`, key)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	aliases := make([]string, 0)
	for rows.Next() {
		var symbol string
		if err := rows.Scan(&symbol); err != nil {
			return nil, err
		}
		aliases = append(aliases, symbol)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(aliases) > 0 {
		for _, symbol := range aliases {
			if slices.Contains(symbols, symbol) {
				continue
			}
			symbols = append(symbols, symbol)
		}
		return symbols, nil
	}

	for _, period := range b.Periods {
		for _, asset := range period.Assets {
			if slices.Contains(symbols, asset.Symbol) {
				continue
			}
			symbols = append(symbols, asset.Symbol)
		}
	}
	return symbols, nil
}

func DownloadFinancialData(ctx context.Context, msg stream.Message) error {
	command := ss.DownloadFinancialDataCommand{}

	err := msg.ParsePayload(&command)
	if err != nil {
		return fmt.Errorf("error unmarshalling DownloadFinancialDataCommand payload: %w", err)
	}

	financeClient, isMarketdataClient := msg.MustGet(dependency.GetMarketdataClient).(financeServicePB.MarketdataClient)
	if !isMarketdataClient {
		return errors.New("error casting marketdata client")
	}
	db := msg.MustGet(stream.DBDep).(*pgxpool.Pool)
	backtests := repository.Backtest{Conn: db}
	b, backtestID, err := loadStoredBacktest(ctx, db, command.BacktestName, command.OwnerID, command.BacktestID)
	if err != nil {
		return fmt.Errorf("error getting backtest: %w", err)
	}

	symbols, err := ingestSymbols(ctx, db, b, backtestID)
	if err != nil {
		return fmt.Errorf("error listing ingest symbols: %w", err)
	}

	provider := strings.TrimSpace(b.GetProvider())
	if provider == "" {
		provider = supplier.DefaultProvider
	}
	if err := finance.ValidatePriceSource(provider, b.GetProfile()); err != nil {
		return fmt.Errorf("price source: %w", err)
	}

	startDate := b.Periods[0].StartDate
	endDate := b.Periods[len(b.Periods)-1].EndDate
	rsp, err := financeClient.DownloadHistoricalData(ctx, &financeServicePB.DownloadHistoricalDataRequest{
		Symbols:   symbols,
		StartDate: startDate,
		EndDate:   endDate,
		Provider:  provider,
		Profile:   b.GetProfile(),
	})
	if err != nil {
		return discardFailedBacktest(ctx, backtests, command.BacktestName, backtestID, err)
	}
	for {
		msg, err := rsp.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			if blocked := blockedDownload(err); blocked != nil {
				discardBlockedBacktest(ctx, backtests, command.BacktestName, backtestID)
				return blocked
			}
			return discardFailedBacktest(ctx, backtests, command.BacktestName, backtestID, err)
		}
		if msg.GetStatus() == financeServicePB.DownloadHistoricalDataResponse_STATUS_ERROR {
			// Any per-symbol download failure must fail create. Disabling the
			// member and exiting 0 left an incomplete universe with no warning.
			return discardSymbolDownloadError(ctx, backtests, command.BacktestName, backtestID,
				provider, msg.Symbol, startDate, endDate, msg.GetError())
		}
	}
	return nil
}

// isNoPriceDataError reports the finance service's no-bars STATUS_ERROR.
func isNoPriceDataError(text string) bool {
	return strings.HasPrefix(text, "no price data for ")
}

// refusedPriceError and refusedVolumeError match the finance service's
// sentences for a bar its storage refused. Only these exact shapes are shown;
// other raw text is not.
var (
	refusedPriceError  = regexp.MustCompile(`^(yahoo|stooq|alpaca) sent an invalid price for [A-Za-z0-9.^=_-]{1,32} on \d{4}-\d{2}-\d{2} \(0, negative or not a number\)$`)
	refusedVolumeError = regexp.MustCompile(`^(yahoo|stooq|alpaca) sent an invalid volume for [A-Za-z0-9.^=_-]{1,32} on \d{4}-\d{2}-\d{2} \(out of range\)$`)
)

// isRefusedBarError reports the finance service's refused-bar sentence.
func isRefusedBarError(text string) bool {
	return refusedPriceError.MatchString(text) || refusedVolumeError.MatchString(text)
}

// providerFaultNextStep ends a sentence about bad provider data. Since #209 a
// bad bar is normally dropped and filled, so reaching this means an adapter
// missed one; a plain retry won't help.
const providerFaultNextStep = "The backtest wasn't saved. Try another provider with --provider, " +
	"or report it at https://github.com/lhjnilsson/foreverbull/issues."

// noPriceDataNextStep ends the sentence when a provider returned no bars. The
// finance service already retried an empty download once, so a result that is
// still empty is most often a wrong or delisted symbol; checking it comes first.
const noPriceDataNextStep = "Nothing was saved. Check that %s traded between those dates, try again later, " +
	"or use another provider with --provider."

// discardSymbolDownloadError deletes the backtest when a symbol's download
// returns STATUS_ERROR. The create must not keep a universe that silently
// omits a member. The lead names the symbol; raw finance/gRPC text stays out
// of the user sentence. "Nothing was saved" or "wasn't saved" only when the
// delete worked.
func discardSymbolDownloadError(ctx context.Context, backtests repository.Backtest, name, id, provider, symbol string, start, end *pb.Date, errText string) error {
	err := deleteStoredBacktest(ctx, backtests, name, id)
	removed := err == nil || errors.Is(err, pgx.ErrNoRows)
	if !removed {
		log.Warn().Err(err).Str("backtest", name).Msg("error deleting backtest after download error")
	}
	var lead string
	nextStep := "Nothing was saved, so run `fbull backtest create` again once it's fixed."
	generic := false
	if isNoPriceDataError(errText) {
		startText := pb.DateToDateString(start)
		endText := pb.DateToDateString(end)
		lead = fmt.Sprintf("%s returned no prices for %s between %s and %s.", strings.ToLower(provider), symbol, startText, endText)
		nextStep = fmt.Sprintf(noPriceDataNextStep, symbol)
	} else if isRefusedBarError(errText) {
		lead = errText + "."
		nextStep = providerFaultNextStep
	} else {
		generic = true
		lead = fmt.Sprintf("couldn't download prices for %s.", symbol)
	}
	if removed && generic {
		return plainSentence(fmt.Sprintf(
			"Couldn't load data for backtest '%s': the price download failed for %s. The backtest wasn't saved. "+
				"Try again in a few minutes, or use another provider with --provider.", name, symbol))
	}
	if removed {
		return plainSentence(fmt.Sprintf(
			"Couldn't load data for backtest '%s': %s %s",
			name, lead, nextStep))
	}
	return plainSentence(fmt.Sprintf(
		"Couldn't load data for backtest '%s': %s The backtest couldn't be removed, so run `fbull backtest delete %s` before you run `fbull backtest create` again.",
		name, lead, name))
}

// blockedDownload returns the finance service's sentence when the price
// provider blocked the download, or nil. The service uses FailedPrecondition
// only for that, and the sentence is the whole error, not wrapped in gRPC
// detail.
func blockedDownload(err error) error {
	if st, ok := status.FromError(err); ok && st.Code() == codes.FailedPrecondition {
		return errors.New(st.Message())
	}
	return nil
}

// discardBlockedBacktest deletes the backtest that the CREATING step stored,
// so retrying the same create with the other provider is not refused as a
// duplicate name. A block leaves nothing that belongs to the backtest: bars
// are stored per provider and symbol, not per backtest. Per-symbol
// STATUS_ERROR also removes the backtest (see discardSymbolDownloadError).
func discardBlockedBacktest(ctx context.Context, backtests repository.Backtest, name, id string) {
	if err := deleteStoredBacktest(ctx, backtests, name, id); err != nil {
		log.Warn().Err(err).Str("backtest", name).Msg("error deleting blocked backtest")
	}
}

// discardFailedBacktest deletes the backtest after a stream or connection
// error from the finance service. We can't tell which bars were stored, so
// the backtest record, its periods and its assets are removed, and a retry
// under the same name works. Bars are stored per provider and symbol, not per
// backtest, so they are kept. The returned error is the step's whole message.
func discardFailedBacktest(ctx context.Context, backtests repository.Backtest, name, id string, cause error) error {
	reason := cause.Error()
	if st, ok := status.FromError(cause); ok && st.Message() != "" {
		reason = st.Message()
	}
	reason = strings.TrimRight(strings.TrimSpace(reason), ".")
	if err := deleteStoredBacktest(ctx, backtests, name, id); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		log.Warn().Err(err).Str("backtest", name).Msg("error deleting backtest after failed download")
		return plainSentence(fmt.Sprintf("Downloading prices failed: %s. The backtest %s could not be removed; "+
			"delete it with `fbull backtest delete %s` before you retry.", reason, name, name))
	}
	if reason != "" {
		runes := []rune(reason)
		runes[0] = unicode.ToLower(runes[0])
		reason = string(runes)
	}
	return plainSentence(fmt.Sprintf("Downloading prices failed: %s. Nothing was changed. Run the same create again.", reason))
}

func Ingest(ctx context.Context, msg stream.Message) error {
	command := ss.IngestCommand{}
	err := msg.ParsePayload(&command)
	if err != nil {
		return fmt.Errorf("error unmarshalling Ingest payload: %w", err)
	}

	db := msg.MustGet(stream.DBDep).(*pgxpool.Pool)
	backtests := repository.Backtest{Conn: db}
	b, backtestID, err := loadStoredBacktest(ctx, db, command.BacktestName, command.OwnerID, command.BacktestID)
	if err != nil {
		return fmt.Errorf("error getting backtest: %w", err)
	}

	store, isStorage := msg.MustGet(stream.StorageDep).(storage.Storage)
	if !isStorage {
		return errors.New("error casting storage")
	}

	metadata := map[string]string{}
	// The two setup failures below are wrapped with %v, not %w, so only the
	// data service's own gRPC status can select the broken-install copy.
	objectKey := b.Name
	if environment.GetAuthEnabled() {
		objectKey = backtestID
	}
	object, err := store.CreateObject(ctx, storage.IngestionsBucket, objectKey, storage.WithMetadata(metadata))
	if err != nil {
		return discardFailedIngest(ctx, backtests, b.Name, backtestID, fmt.Errorf("error creating ingestion: %v", err))
	}

	ze, err := msg.Call(ctx, dependency.GetEngineKey)
	if err != nil {
		return discardFailedIngest(ctx, backtests, b.Name, backtestID, fmt.Errorf("error getting zipline engine: %v", err))
	}

	engine, isEngine := ze.(engine.Engine)
	if !isEngine {
		return errors.New("error casting zipline engine")
	}

	notices, err := engine.Ingest(ctx, b, object)
	if err != nil {
		return discardFailedIngest(ctx, backtests, b.Name, backtestID, err)
	}
	if len(notices) > 0 {
		// The backtest is kept. The notice rides on the completed INGESTING
		// step, so the CLI prints which prices were filled.
		return &stream.Notice{Lines: notices}
	}
	return nil
}
