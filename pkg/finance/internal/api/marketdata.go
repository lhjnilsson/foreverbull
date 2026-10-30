package api

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/internal/numfmt"
	"github.com/lhjnilsson/foreverbull/internal/redact"
	"github.com/lhjnilsson/foreverbull/pkg/finance/internal/repository"
	"github.com/lhjnilsson/foreverbull/pkg/finance/internal/suppliers/marketdata"
	"github.com/lhjnilsson/foreverbull/pkg/finance/internal/ticker"
	"github.com/lhjnilsson/foreverbull/pkg/finance/supplier"
	internal_pb "github.com/lhjnilsson/foreverbull/pkg/pb"
	pb "github.com/lhjnilsson/foreverbull/pkg/pb/finance"
	servicePb "github.com/lhjnilsson/foreverbull/pkg/pb/finance/service"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type MarketdataServer struct {
	servicePb.UnimplementedMarketdataServer

	pgx        *pgxpool.Pool
	marketdata supplier.Marketdata
}

func NewMarketdataServer(pgx *pgxpool.Pool, md supplier.Marketdata) *MarketdataServer {
	return &MarketdataServer{
		pgx:        pgx,
		marketdata: md,
	}
}

func (fs *MarketdataServer) GetAsset(ctx context.Context, req *servicePb.GetAssetRequest) (*servicePb.GetAssetResponse, error) {
	identifier := req.GetIdentifier()
	if identifier == nil {
		return nil, fmt.Errorf("identifier is nil")
	}
	var asset *pb.Asset
	var err error
	switch identifier := identifier.(type) {
	case *servicePb.GetAssetRequest_Symbol:
		asset, err = fs.marketdata.GetAsset(ctx, identifier.Symbol)
		if err != nil {
			return nil, fmt.Errorf("error getting asset: %w", redact.URLError(err))
		}
	case *servicePb.GetAssetRequest_Isin:
		assets, err := fs.marketdata.SearchAsset(ctx, identifier.Isin)
		if err != nil {
			return nil, fmt.Errorf("error getting asset: %w", redact.URLError(err))
		}
		if len(assets) == 0 {
			return nil, fmt.Errorf("asset not found by isin: %s", identifier.Isin)
		}
		for _, a := range assets {
			if a.ISIN == identifier.Isin {
				asset = a
				break
			}
		}
	default:
		return nil, fmt.Errorf("unknown identifier type: %T", identifier)
	}
	if asset == nil {
		return nil, fmt.Errorf("asset not found: %s", req.GetSymbol())
	}

	return &servicePb.GetAssetResponse{
		Asset: asset,
	}, nil
}

func (fs *MarketdataServer) GetIndex(ctx context.Context, req *servicePb.GetIndexRequest) (*servicePb.GetIndexResponse, error) {
	assets, err := fs.marketdata.GetIndex(ctx, req.GetSymbol())
	if err != nil {
		return nil, fmt.Errorf("error getting index: %w", redact.URLError(err))
	}

	return &servicePb.GetIndexResponse{
		Assets: assets,
	}, nil
}

const downloadWorkers = 10

// downloadTimeLimit bounds one DownloadHistoricalData. At the shared provider
// rates a 10,000-symbol download takes about an hour on Yahoo or Alpaca and
// about 85 minutes on Stooq, so this leaves room for 429 backoffs while a
// hung provider still can't hold the request and its workers forever.
// Symbols stored before the limit stay stored. Backtest create names it as
// the 2-hour limit, so change that copy with it. Tests shorten it.
var downloadTimeLimit = 2 * time.Hour

// errDownloadTimeLimit is the ctx cause when downloadTimeLimit ends a
// download, so it is told apart from the client's own deadline or cancel.
var errDownloadTimeLimit = errors.New("download time limit reached")

// metadataProvider is the provider GetAsset asks, whatever the request's
// price provider: asset metadata stays on Yahoo.
const metadataProvider = "yahoo"

// downloadSymbols returns the request's symbols under their stored names
// (marketdata.StoredSymbol: BRK.B, BRK-B and BRKB are BRKB) with repeats left
// out, in request order, so each symbol is fetched, stored and reported once.
//
// More than marketdata.MaxDownloadSymbols distinct symbols is
// InvalidArgument; counting stops at the first symbol over the cap. Nothing
// is fetched or stored then. The caller has already refused a symbol outside
// the ticker pattern (blank included) by its position.
func downloadSymbols(symbols []string) ([]string, error) {
	// Distinct stored names, the count marketdata.DownloadSymbolCount gives
	// backtest create.
	seen := make(map[string]struct{}, min(len(symbols), marketdata.MaxDownloadSymbols+1))
	out := make([]string, 0, min(len(symbols), marketdata.MaxDownloadSymbols))
	for _, symbol := range symbols {
		name := marketdata.StoredSymbol(symbol)
		if _, ok := seen[name]; ok {
			continue
		}
		if len(out) == marketdata.MaxDownloadSymbols {
			return nil, status.Error(codes.InvalidArgument, tooManySymbolsMessage)
		}
		seen[name] = struct{}{}
		out = append(out, name)
	}
	return out, nil
}

// tooManySymbolsMessage refuses a request over the per-request cap. Backtest
// create counts the same way before it stores a backtest and shows its own
// sentence with the count; this one is for other callers of the RPC.
var tooManySymbolsMessage = fmt.Sprintf(
	"one download can carry at most %s distinct symbols, and this one has more. Split it into smaller downloads.",
	numfmt.Thousands(marketdata.MaxDownloadSymbols))

// downloadSender serialises Send on the download stream. gRPC doesn't allow
// concurrent SendMsg on one stream, and every download worker reports here.
// The first failed Send calls stop: the client can't get any later result,
// so the workers must not keep fetching and storing for it.
type downloadSender struct {
	mu     sync.Mutex
	stream servicePb.Marketdata_DownloadHistoricalDataServer
	stop   context.CancelFunc
	err    error
	// reported holds every symbol sent, at most MaxDownloadSymbols, so a
	// download stopped by its time limit can name one that didn't finish.
	reported map[string]struct{}
}

// newDownloadSender wires stop, which cancels the download's workers.
func newDownloadSender(stream servicePb.Marketdata_DownloadHistoricalDataServer, stop context.CancelFunc) *downloadSender {
	return &downloadSender{stream: stream, stop: stop}
}

func (s *downloadSender) send(rsp *servicePb.DownloadHistoricalDataResponse) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return
	}
	if err := s.stream.Send(rsp); err == nil {
		if s.reported == nil {
			s.reported = make(map[string]struct{})
		}
		s.reported[rsp.GetSymbol()] = struct{}{}
	} else {
		log.Err(err).Msg("error sending response, stopping the download")
		s.err = err
		// A sender built without newDownloadSender has no stop; it still
		// records the failure and sends nothing more instead of panicking.
		if s.stop != nil {
			s.stop()
		}
	}
}

// firstUnreported is the first of symbols, in request order, that no
// response was sent for, or "" when every one was.
func (s *downloadSender) firstUnreported(symbols []string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, symbol := range symbols {
		if _, ok := s.reported[symbol]; !ok {
			return symbol
		}
	}
	return ""
}

// sendErr is the first failed Send, or nil.
func (s *downloadSender) sendErr() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.err
}

// downloadFailedError is the generic STATUS_ERROR text for a failure whose
// details (DB, driver or provider text) must stay in the log. The RPC is
// reachable by editors, so raw internal errors don't go out.
func downloadFailedError(symbol string) error {
	return fmt.Errorf("couldn't download prices for %s. Try again in a few minutes, "+ //nolint:staticcheck // ST1005: agreed user-facing copy ends with a period
		"or use another provider with --provider.", symbol)
}

// internalDownloadError logs err with context and returns the generic text.
func internalDownloadError(symbol, step string, err error) error {
	log.Error().Err(redact.URLError(err)).Str("symbol", symbol).Str("step", step).Msg("historical data download failed")
	return downloadFailedError(symbol)
}

// noPriceDataError is the STATUS_ERROR text when no ticker of the instrument
// has bars. DownloadFinancialData uses this prefix for the no-bars sentence;
// any STATUS_ERROR fails create.
func noPriceDataError(symbol string) error {
	return fmt.Errorf("%w for %s", errNoPriceData, symbol)
}

var errNoPriceData = errors.New("no price data")

// ambiguousSymbolError is the STATUS_ERROR text when the asked ticker maps to
// more than one instrument. provider is the normalised provider and symbol
// passed the ticker check; the backtest service matches this exact shape.
func ambiguousSymbolError(provider, symbol string) error {
	return fmt.Errorf("%s has more than one instrument for %s", provider, symbol)
}

// providerTimedOut reports a provider call that ran out of time while the
// download itself still had time. A database error is never one, even when
// its driver says timeout: that stays the store failure it is.
func providerTimedOut(ctx context.Context, err error) bool {
	if ctx.Err() != nil {
		return false
	}
	var stored *repository.StoreError
	if errors.As(err, &stored) {
		return false
	}
	return marketdata.IsProviderTimeout(err)
}

func failDownload(rsp *servicePb.DownloadHistoricalDataResponse, err error) {
	rsp.Status = servicePb.DownloadHistoricalDataResponse_STATUS_ERROR
	rsp.Error = err.Error()
}

// failSymbol reports a symbol as STATUS_ERROR. Create fails on that status.
// After a block cancels the download, an in-flight symbol fails only because
// of that cancel, so it is not reported and stays enabled for the retry.
func failSymbol(ctx context.Context, sender *downloadSender,
	rsp *servicePb.DownloadHistoricalDataResponse, err error,
) {
	if ctx.Err() != nil {
		return
	}
	failDownload(rsp, err)
	sender.send(rsp)
}

func (fs *MarketdataServer) downloadSymbol(
	ctx context.Context,
	sender *downloadSender,
	symbol string,
	start time.Time,
	end *time.Time,
	provider string,
	assetRepo repository.Asset,
	ohlcRepo repository.OHLC,
	bars supplier.Marketdata,
) *marketdata.BlockedError {
	rsp := &servicePb.DownloadHistoricalDataResponse{
		Symbol: symbol,
		Status: servicePb.DownloadHistoricalDataResponse_STATUS_OK,
	}

	asset, err := fs.marketdata.GetAsset(ctx, symbol)
	if err != nil {
		if providerTimedOut(ctx, err) {
			failSymbol(ctx, sender, rsp, &marketdata.TimeoutError{Provider: metadataProvider, Symbol: symbol})
			return nil
		}
		failSymbol(ctx, sender, rsp, internalDownloadError(symbol, "get asset", err))
		return nil
	}
	// The provider's ticker becomes the stored asset, the bars' key, and the
	// symbol in a refused-bar sentence, so it is checked like a request one.
	if !ticker.Valid(asset.GetSymbol()) {
		log.Warn().Str("symbol", symbol).Str("provider_symbol", ticker.Logged(asset.GetSymbol())).
			Msg("provider returned a ticker outside the symbol pattern")
		failSymbol(ctx, sender, rsp, downloadFailedError(symbol))
		return nil
	}

	if err := assetRepo.Store(ctx, asset.Symbol, asset.Name); err != nil {
		failSymbol(ctx, sender, rsp, internalDownloadError(symbol, "store asset", err))
		return nil
	}

	storeSymbol := asset.Symbol
	ohlcs, err := bars.GetOHLC(ctx, storeSymbol, start, end)
	if err != nil {
		var blocked *marketdata.BlockedError
		if errors.As(err, &blocked) {
			return blocked
		}
		// A provider that didn't answer in time would likely not answer for
		// a later ticker either, so the symbol fails with the timeout.
		if providerTimedOut(ctx, err) {
			failSymbol(ctx, sender, rsp, &marketdata.TimeoutError{Provider: provider, Symbol: symbol})
			return nil
		}
	}
	if err != nil || len(ohlcs) == 0 {
		// Yahoo sometimes answers a listed symbol (BK) with no data. Ask once
		// more before treating it as renamed or missing.
		ohlcs, err = retryOHLC(ctx, bars, storeSymbol, start, end)
		if err != nil {
			var blocked *marketdata.BlockedError
			if errors.As(err, &blocked) {
				return blocked
			}
		}
	}
	if err != nil || len(ohlcs) == 0 {
		fetched, fetchErr := fs.fetchBarsViaLaterTicker(ctx, symbol, start, end, bars, assetRepo)
		if fetchErr != nil {
			var blocked *marketdata.BlockedError
			if errors.As(fetchErr, &blocked) {
				return blocked
			}
			// Only the no-bars sentinel selects the "no price data for …
			// between …" sentence. A DB error in Resolve or ListAliases says
			// nothing about the prices, so it is logged and the user gets
			// the generic download failure.
			if errors.Is(fetchErr, errNoPriceData) {
				failSymbol(ctx, sender, rsp, noPriceDataError(symbol))
				return nil
			}
			if errors.Is(fetchErr, repository.ErrAmbiguousSymbol) {
				failSymbol(ctx, sender, rsp, ambiguousSymbolError(provider, symbol))
				return nil
			}
			if ctx.Err() != nil {
				return nil
			}
			failSymbol(ctx, sender, rsp, internalDownloadError(symbol, "find later ticker", fetchErr))
			return nil
		}
		storeSymbol = fetched.symbol
		ohlcs = fetched.bars
	}

	// The same overlap and re-scale rule as ingest, in one locked transaction
	// per symbol: a refused bar or a DB error stores none of them, and stored
	// days after a split are re-scaled instead of left next to new ones. Only
	// the refused-bar sentence goes to the client as is. The asset row above
	// and a later ticker's asset and asset_symbol rows are written outside
	// this transaction and stay when the bars roll back; a retry finds them
	// and stores the bars again.
	fetch := prefetchedBars(ctx, bars, storeSymbol, start, end, ohlcs)
	if err := ohlcRepo.Sync(ctx, provider, storeSymbol, start, end, fetch); err != nil {
		var blocked *marketdata.BlockedError
		if errors.As(err, &blocked) {
			return blocked
		}
		var invalid *repository.InvalidPriceError
		var stored *repository.StoreError
		switch {
		case providerTimedOut(ctx, err):
			// A refetch under the symbol lock ran out of time. It ends before
			// the transaction's idle timeout, so it is told as the provider's
			// delay, not as a failure to store.
			err = &marketdata.TimeoutError{Provider: provider, Symbol: symbol}
		case errors.As(err, &invalid):
		case errors.Is(err, repository.ErrNoBars):
			err = noPriceDataError(symbol)
		case errors.As(err, &stored):
			err = internalDownloadError(symbol, "store bars", stored.Err)
		default:
			err = internalDownloadError(symbol, "refetch bars", err)
		}
		failSymbol(ctx, sender, rsp, err)
		return nil
	}

	sender.send(rsp)
	return nil
}

// ohlcRetryDelay is the pause before asking a provider again after it
// returned no bars.
const ohlcRetryDelay = 250 * time.Millisecond

// retryOHLC asks for the bars a second time after a short pause.
func retryOHLC(ctx context.Context, bars supplier.Marketdata, symbol string,
	start time.Time, end *time.Time,
) ([]*pb.OHLC, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(ohlcRetryDelay):
	}
	return bars.GetOHLC(ctx, symbol, start, end)
}

// prefetchedBars is the fetch Sync uses for one download. The bars already
// fetched for start through end serve any request inside that span (Sync
// asks from the last stored day for an incremental fetch); anything else,
// such as a refetch from an earlier stored day, goes to the provider.
func prefetchedBars(ctx context.Context, bars supplier.Marketdata, symbol string, start time.Time, end *time.Time,
	fetched []*pb.OHLC,
) repository.FetchBars {
	return func(from time.Time, to *time.Time) ([]*pb.OHLC, error) {
		if !withinSpan(start, end, from, to) {
			return bars.GetOHLC(ctx, symbol, from, to)
		}
		out := make([]*pb.OHLC, 0, len(fetched))
		for _, b := range fetched {
			day := b.GetTimestamp().AsTime().UTC().Format(time.DateOnly)
			if day < from.UTC().Format(time.DateOnly) {
				continue
			}
			if to != nil && day > to.UTC().Format(time.DateOnly) {
				continue
			}
			out = append(out, b)
		}
		return out, nil
	}
}

// withinSpan reports whether the trading days from through to lie inside
// start through end. A nil end is open (through today).
func withinSpan(start time.Time, end *time.Time, from time.Time, to *time.Time) bool {
	day := func(t time.Time) string { return t.UTC().Format(time.DateOnly) }
	if day(from) < day(start) {
		return false
	}
	if end == nil {
		return true
	}
	return to != nil && day(*to) <= day(*end)
}

type fetchedBars struct {
	symbol string
	bars   []*pb.OHLC
}

// fetchBarsViaLaterTicker loads bars under another ticker of the same instrument
// when the asked ticker has none. Yahoo keeps a renamed company's history only
// under the current ticker. Candidates come from asset_symbol, then from a
// search by the instrument's identity. Bars are stored under the ticker that
// returned them; asset_symbol gains that ticker when it was not stored yet.
func (fs *MarketdataServer) fetchBarsViaLaterTicker(
	ctx context.Context,
	symbol string,
	start time.Time,
	end *time.Time,
	bars supplier.Marketdata,
	assetRepo repository.Asset,
) (*fetchedBars, error) {
	instruments := repository.Instruments{Conn: fs.pgx}
	inst, err := instruments.Resolve(ctx, symbol, start)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, noPriceDataError(symbol)
		}
		return nil, err
	}

	candidates, err := laterTickerCandidates(ctx, instruments, fs.marketdata, inst.ISIN, symbol)
	if err != nil {
		return nil, err
	}
	if len(candidates) == 0 {
		return nil, noPriceDataError(symbol)
	}

	for _, candidate := range candidates {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		ohlcs, err := bars.GetOHLC(ctx, candidate, start, end)
		if err != nil {
			var blocked *marketdata.BlockedError
			if errors.As(err, &blocked) {
				return nil, blocked
			}
			continue
		}
		if len(ohlcs) == 0 {
			continue
		}

		if a, err := fs.marketdata.GetAsset(ctx, candidate); err == nil {
			_ = assetRepo.Store(ctx, a.Symbol, a.Name)
		} else {
			_ = assetRepo.Store(ctx, candidate, candidate)
		}

		laterFrom := start
		if end != nil {
			laterFrom = end.AddDate(0, 0, 1)
		} else {
			laterFrom = time.Now().UTC().AddDate(0, 0, 1)
		}
		if err := instruments.EnsureLaterAlias(ctx, inst.ISIN, symbol, candidate, laterFrom); err != nil {
			log.Warn().Err(err).Str("isin", inst.ISIN).Str("symbol", candidate).
				Msg("could not record later ticker after fetching its bars")
		}
		return &fetchedBars{symbol: candidate, bars: ohlcs}, nil
	}
	return nil, noPriceDataError(symbol)
}

func laterTickerCandidates(
	ctx context.Context,
	instruments repository.Instruments,
	md supplier.Marketdata,
	isin, asked string,
) ([]string, error) {
	asked = strings.TrimSpace(asked)
	seen := map[string]struct{}{asked: {}}
	candidates := make([]string, 0)

	aliases, err := instruments.ListAliases(ctx, isin)
	if err != nil {
		return nil, err
	}
	for _, alias := range aliases {
		symbol := strings.TrimSpace(alias.Symbol)
		if !candidateSymbol(isin, symbol) {
			continue
		}
		if _, ok := seen[symbol]; ok {
			continue
		}
		seen[symbol] = struct{}{}
		candidates = append(candidates, symbol)
	}

	found, err := md.SearchAsset(ctx, isin)
	if err != nil {
		log.Info().Err(redact.URLError(err)).Str("isin", isin).Msg("search for current ticker failed")
	} else {
		for _, a := range found {
			symbol := strings.TrimSpace(a.Symbol)
			if !candidateSymbol(isin, symbol) {
				continue
			}
			if a.ISIN != "" && a.ISIN != isin {
				continue
			}
			if _, ok := seen[symbol]; ok {
				continue
			}
			seen[symbol] = struct{}{}
			candidates = append(candidates, symbol)
		}
	}
	return candidates, nil
}

// candidateSymbol reports whether a later-ticker candidate may be fetched.
// Bars and the alias are stored under it and a refused bar names it, so a
// stored alias or a search result outside the ticker pattern is skipped.
func candidateSymbol(isin, symbol string) bool {
	if symbol == "" {
		return false
	}
	if ticker.Valid(symbol) {
		return true
	}
	log.Warn().Str("isin", isin).Str("candidate", ticker.Logged(symbol)).Msg("skipping a later ticker outside the symbol pattern")
	return false
}

func (fs *MarketdataServer) DownloadHistoricalData(req *servicePb.DownloadHistoricalDataRequest, serverStream servicePb.Marketdata_DownloadHistoricalDataServer) error {
	ctx := serverStream.Context()
	if err := requireTradingEditor(ctx); err != nil {
		return err
	}
	// Every symbol is echoed in a response, so all are checked before any
	// download starts. The first invalid one is named by position only.
	symbols := req.GetSymbols()
	for index, symbol := range symbols {
		if !ticker.Valid(symbol) {
			return status.Error(codes.InvalidArgument, ticker.InvalidMessage(index+1, len(symbols)))
		}
	}

	symbols, err := downloadSymbols(symbols)
	if err != nil {
		return err
	}

	provider, err := marketdata.NormalizeProvider(req.GetProvider())
	if err != nil {
		return err
	}
	profilesFile := environment.GetTradingProfilesFile()
	if err := marketdata.ValidatePriceSource(provider, req.GetProfile(), profilesFile); err != nil {
		return err
	}
	bars, err := marketdata.OHLCClient(provider, req.GetProfile(), profilesFile, fs.marketdata)
	if err != nil {
		return err
	}

	start := internal_pb.DateToTime(req.GetStartDate())
	var end *time.Time
	if req.GetEndDate() != nil {
		e := internal_pb.DateToTime(req.GetEndDate())
		end = &e
	}

	assetRepo := repository.Asset{Conn: fs.pgx}
	ohlcRepo := repository.OHLC{Conn: fs.pgx}

	// The time limit ends the whole download. A symbol stored before it stays
	// stored; one still in flight rolls back and is named in the timeout.
	ctx, stopAtLimit := context.WithTimeoutCause(ctx, downloadTimeLimit, errDownloadTimeLimit)
	defer stopAtLimit()
	// The first blocked symbol or failed Send cancels the rest. Workers that
	// already started finish their symbol, so the error is still returned once.
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	var blockedOnce sync.Once
	var blocked *marketdata.BlockedError

	sender := newDownloadSender(serverStream, cancel)
	var wg sync.WaitGroup
	assetCh := make(chan string, downloadWorkers)
	for range downloadWorkers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for symbol := range assetCh {
				if ctx.Err() != nil {
					continue
				}
				if err := fs.downloadSymbol(ctx, sender, symbol, start, end, provider, assetRepo, ohlcRepo, bars); err != nil {
					blockedOnce.Do(func() {
						blocked = err
						cancel()
					})
				}
			}
		}()
	}

feed:
	for _, symbol := range symbols {
		select {
		case assetCh <- symbol:
		case <-ctx.Done():
			break feed
		}
	}
	close(assetCh)
	wg.Wait()

	if blocked != nil {
		return status.Error(codes.FailedPrecondition, blocked.Error())
	}
	if err := sender.sendErr(); err != nil {
		return err
	}
	if errors.Is(context.Cause(ctx), errDownloadTimeLimit) {
		if symbol := sender.firstUnreported(symbols); symbol != "" {
			log.Warn().Str("provider", provider).Str("symbol", symbol).Dur("limit", downloadTimeLimit).
				Msg("download stopped at its time limit")
			return status.Error(codes.DeadlineExceeded, (&marketdata.TimeLimitError{Symbol: symbol}).Error())
		}
		return nil
	}
	return ctx.Err()
}

func (fs *MarketdataServer) GetOHLC(ctx context.Context, req *servicePb.GetOHLCRequest) (*servicePb.GetOHLCResponse, error) {
	symbols := req.GetSymbols()
	if len(symbols) == 0 {
		return nil, status.Error(codes.InvalidArgument, "symbols is required")
	}
	if req.GetAsOf() == nil {
		return nil, status.Error(codes.InvalidArgument, "as_of is required")
	}
	provider, err := marketdata.NormalizeProvider(req.GetProvider())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	limit := int(req.GetLimit())
	ohlcRepo := repository.OHLC{Conn: fs.pgx}
	bars, err := ohlcRepo.List(ctx, provider, symbols, req.GetAsOf().AsTime(), limit)
	if err != nil {
		msg := err.Error()
		if strings.Contains(msg, "symbols:") || strings.Contains(msg, "limit:") {
			return nil, status.Error(codes.InvalidArgument, msg)
		}
		return nil, fmt.Errorf("list ohlc: %w", err)
	}
	return &servicePb.GetOHLCResponse{Bars: bars}, nil
}
