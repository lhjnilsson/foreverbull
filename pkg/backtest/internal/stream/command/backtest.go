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

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/storage"
	"github.com/lhjnilsson/foreverbull/internal/stream"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/engine"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/repository"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/stream/dependency"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/universe"
	ss "github.com/lhjnilsson/foreverbull/pkg/backtest/stream"
	"github.com/lhjnilsson/foreverbull/pkg/pb"
	"github.com/lhjnilsson/foreverbull/pkg/pb/backtest"
	financeServicePB "github.com/lhjnilsson/foreverbull/pkg/pb/finance/service"
	"github.com/rs/zerolog/log"
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

	db := msg.MustGet(stream.DBDep).(*pgxpool.Pool)
	backtests := repository.Backtest{Conn: db}
	_, err = backtests.Create(ctx, &b)
	if err != nil {
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
		Periods:   calendarYearPeriods(start, end),
	}
	fillPeriodAssets(b.Periods, result.Aliases, hasData)

	db := msg.MustGet(stream.DBDep).(*pgxpool.Pool)
	backtests := repository.Backtest{Conn: db}
	_, err = backtests.Create(ctx, &b)
	if err != nil {
		return fmt.Errorf("error creating backtest: %w", err)
	}

	if err := persistUniverse(ctx, db, command.BacktestName, snapshots, result); err != nil {
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
	dates, err := scraper.ListSnapshotDates(etf, start, end)
	if err != nil {
		return nil, fmt.Errorf("error listing snapshot dates: %w", err)
	}

	snaps := make([]universe.Snapshot, 0, len(dates))
	for _, asOf := range dates {
		assets, err := scraper.GetSnapshot(etf, asOf)
		if err != nil {
			return nil, err
		}
		holdings := holdingsFromAssets(assets)
		if len(holdings) == 0 {
			continue
		}
		snaps = append(snaps, universe.Snapshot{AsOf: asOf, Holdings: holdings})
	}
	if len(snaps) == 0 {
		return nil, fmt.Errorf("no published ETF snapshots for %s between %s and %s",
			etf, start.Format("2006-01-02"), end.Format("2006-01-02"))
	}
	return snaps, nil
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

func persistUniverse(ctx context.Context, db *pgxpool.Pool, backtestName string, snaps []universe.Snapshot, result universe.Result) error {
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

func ingestSymbols(ctx context.Context, db *pgxpool.Pool, b *backtest.Backtest) ([]string, error) {
	symbols := []string{b.Benchmark}

	rows, err := db.Query(ctx, `
		SELECT DISTINCT s.symbol
		  FROM universe_member m
		  JOIN asset_symbol s ON s.isin = m.isin
		 WHERE m.backtest_name = $1
		 ORDER BY s.symbol`, b.Name)
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
	b, err := backtests.Get(ctx, command.BacktestName)
	if err != nil {
		return fmt.Errorf("error getting backtest: %w", err)
	}

	symbols, err := ingestSymbols(ctx, db, b)
	if err != nil {
		return fmt.Errorf("error listing ingest symbols: %w", err)
	}

	rsp, err := financeClient.DownloadHistoricalData(ctx, &financeServicePB.DownloadHistoricalDataRequest{
		Symbols:   symbols,
		StartDate: b.Periods[0].StartDate,
		EndDate:   b.Periods[len(b.Periods)-1].EndDate,
	})
	if err != nil {
		return fmt.Errorf("error downloading financial data: %w", err)
	}
	for {
		msg, err := rsp.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return fmt.Errorf("error receiving response: %w", err)
		}
		if msg.GetStatus() == financeServicePB.DownloadHistoricalDataResponse_STATUS_ERROR {
			err := backtests.UpdatePeriodAsset(ctx, command.BacktestName, msg.Symbol, false)
			if err != nil {
				return fmt.Errorf("error updating period asset: %w", err)
			}

		}
	}
	return nil
}

func Ingest(ctx context.Context, msg stream.Message) error {
	command := ss.IngestCommand{}
	err := msg.ParsePayload(&command)
	if err != nil {
		return fmt.Errorf("error unmarshalling Ingest payload: %w", err)
	}

	db := msg.MustGet(stream.DBDep).(*pgxpool.Pool)
	backtests := repository.Backtest{Conn: db}
	b, err := backtests.Get(ctx, command.BacktestName)
	if err != nil {
		return fmt.Errorf("error getting backtest: %w", err)
	}

	store, isStorage := msg.MustGet(stream.StorageDep).(storage.Storage)
	if !isStorage {
		return errors.New("error casting storage")
	}

	metadata := map[string]string{}
	object, err := store.CreateObject(ctx, storage.IngestionsBucket, b.Name, storage.WithMetadata(metadata))
	if err != nil {
		return fmt.Errorf("error creating ingestion: %w", err)
	}

	ze, err := msg.Call(ctx, dependency.GetEngineKey)
	if err != nil {
		return fmt.Errorf("error getting zipline engine: %w", err)
	}

	engine, isEngine := ze.(engine.Engine)
	if !isEngine {
		return errors.New("error casting zipline engine")
	}

	err = engine.Ingest(ctx, b, object)
	if err != nil {
		return fmt.Errorf("error ingesting data: %w", err)
	}
	return nil
}
