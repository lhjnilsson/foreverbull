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
	"github.com/lhjnilsson/foreverbull/pkg/finance/internal/repository"
	"github.com/lhjnilsson/foreverbull/pkg/finance/internal/suppliers/marketdata"
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

func (fs *MarketdataServer) GetAsset(_ context.Context, req *servicePb.GetAssetRequest) (*servicePb.GetAssetResponse, error) {
	identifier := req.GetIdentifier()
	if identifier == nil {
		return nil, fmt.Errorf("identifier is nil")
	}
	var asset *pb.Asset
	var err error
	switch identifier := identifier.(type) {
	case *servicePb.GetAssetRequest_Symbol:
		asset, err = fs.marketdata.GetAsset(identifier.Symbol)
		if err != nil {
			return nil, fmt.Errorf("error getting asset: %w", err)
		}
	case *servicePb.GetAssetRequest_Isin:
		assets, err := fs.marketdata.SearchAsset(identifier.Isin)
		if err != nil {
			return nil, fmt.Errorf("error getting asset: %w", err)
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

func (fs *MarketdataServer) GetIndex(_ context.Context, req *servicePb.GetIndexRequest) (*servicePb.GetIndexResponse, error) {
	assets, err := fs.marketdata.GetIndex(req.GetSymbol())
	if err != nil {
		return nil, fmt.Errorf("error getting index: %w", err)
	}

	return &servicePb.GetIndexResponse{
		Assets: assets,
	}, nil
}

const downloadWorkers = 10

func sendDownloadResponse(serverStream servicePb.Marketdata_DownloadHistoricalDataServer, rsp *servicePb.DownloadHistoricalDataResponse) {
	if err := serverStream.Send(rsp); err != nil {
		log.Err(err).Msg("error sending response")
	}
}

// noPriceDataError is the STATUS_ERROR text when no ticker of the instrument
// has bars. DownloadFinancialData uses this prefix for the no-bars sentence;
// any STATUS_ERROR fails create.
func noPriceDataError(symbol string) error {
	return fmt.Errorf("no price data for %s", symbol)
}

func failDownload(rsp *servicePb.DownloadHistoricalDataResponse, err error) {
	rsp.Status = servicePb.DownloadHistoricalDataResponse_STATUS_ERROR
	rsp.Error = err.Error()
}

// failSymbol reports a symbol as STATUS_ERROR. Create fails on that status.
// After a block cancels the download, an in-flight symbol fails only because
// of that cancel, so it is not reported and stays enabled for the retry.
func failSymbol(ctx context.Context, serverStream servicePb.Marketdata_DownloadHistoricalDataServer,
	rsp *servicePb.DownloadHistoricalDataResponse, err error,
) {
	if ctx.Err() != nil {
		return
	}
	failDownload(rsp, err)
	sendDownloadResponse(serverStream, rsp)
}

func (fs *MarketdataServer) downloadSymbol(
	ctx context.Context,
	serverStream servicePb.Marketdata_DownloadHistoricalDataServer,
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

	asset, err := fs.marketdata.GetAsset(symbol)
	if err != nil {
		failSymbol(ctx, serverStream, rsp, err)
		return nil
	}

	if err := assetRepo.Store(ctx, asset.Symbol, asset.Name); err != nil {
		failSymbol(ctx, serverStream, rsp, err)
		return nil
	}

	storeSymbol := asset.Symbol
	ohlcs, err := bars.GetOHLC(storeSymbol, start, end)
	if err != nil {
		var blocked *marketdata.BlockedError
		if errors.As(err, &blocked) {
			return blocked
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
			// no-bars sentinel; create fails on any STATUS_ERROR, and this
			// prefix selects the "no price data for … between …" sentence.
			failSymbol(ctx, serverStream, rsp, noPriceDataError(symbol))
			return nil
		}
		storeSymbol = fetched.symbol
		ohlcs = fetched.bars
	}

	// Check the whole batch first, so a refused bar stores none of them.
	if err := repository.ValidateBars(provider, storeSymbol, ohlcs); err != nil {
		failSymbol(ctx, serverStream, rsp, err)
		return nil
	}
	for _, ohlc := range ohlcs {
		if err := ohlcRepo.Store(ctx, provider, storeSymbol, ohlc.Timestamp.AsTime(),
			ohlc.Open, ohlc.High, ohlc.Low, ohlc.Close, int(ohlc.Volume)); err != nil {
			failSymbol(ctx, serverStream, rsp, err)
			return nil
		}
	}

	sendDownloadResponse(serverStream, rsp)
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
	return bars.GetOHLC(symbol, start, end)
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
		ohlcs, err := bars.GetOHLC(candidate, start, end)
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

		if a, err := fs.marketdata.GetAsset(candidate); err == nil {
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
		if symbol == "" {
			continue
		}
		if _, ok := seen[symbol]; ok {
			continue
		}
		seen[symbol] = struct{}{}
		candidates = append(candidates, symbol)
	}

	found, err := md.SearchAsset(isin)
	if err != nil {
		log.Info().Err(err).Str("isin", isin).Msg("search for current ticker failed")
	} else {
		for _, a := range found {
			symbol := strings.TrimSpace(a.Symbol)
			if symbol == "" {
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

func (fs *MarketdataServer) DownloadHistoricalData(req *servicePb.DownloadHistoricalDataRequest, serverStream servicePb.Marketdata_DownloadHistoricalDataServer) error {
	ctx := serverStream.Context()
	if err := requireTradingEditor(ctx); err != nil {
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

	// The first blocked symbol cancels the rest. Workers that already started
	// finish their symbol, so the error is still returned once.
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	var blockedOnce sync.Once
	var blocked *marketdata.BlockedError

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
				if err := fs.downloadSymbol(ctx, serverStream, symbol, start, end, provider, assetRepo, ohlcRepo, bars); err != nil {
					blockedOnce.Do(func() {
						blocked = err
						cancel()
					})
				}
			}
		}()
	}

feed:
	for _, symbol := range req.GetSymbols() {
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
