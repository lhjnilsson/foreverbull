package api

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/pkg/finance/internal/repository"
	"github.com/lhjnilsson/foreverbull/pkg/finance/supplier"
	internal_pb "github.com/lhjnilsson/foreverbull/pkg/pb"
	pb "github.com/lhjnilsson/foreverbull/pkg/pb/finance"
	servicePb "github.com/lhjnilsson/foreverbull/pkg/pb/finance/service"
	"github.com/rs/zerolog/log"
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

func failDownload(rsp *servicePb.DownloadHistoricalDataResponse, err error) {
	rsp.Status = servicePb.DownloadHistoricalDataResponse_STATUS_ERROR
	rsp.Error = err.Error()
}

func (fs *MarketdataServer) downloadSymbol(
	ctx context.Context,
	serverStream servicePb.Marketdata_DownloadHistoricalDataServer,
	symbol string,
	start time.Time,
	end *time.Time,
	assetRepo repository.Asset,
	ohlcRepo repository.OHLC,
) {
	rsp := &servicePb.DownloadHistoricalDataResponse{
		Symbol: symbol,
		Status: servicePb.DownloadHistoricalDataResponse_STATUS_OK,
	}

	asset, err := fs.marketdata.GetAsset(symbol)
	if err != nil {
		failDownload(rsp, err)
		sendDownloadResponse(serverStream, rsp)
		return
	}

	if err := assetRepo.Store(ctx, asset.Symbol, asset.Name); err != nil {
		failDownload(rsp, err)
		sendDownloadResponse(serverStream, rsp)
		return
	}

	ohlcs, err := fs.marketdata.GetOHLC(asset.Symbol, start, end)
	if err != nil {
		failDownload(rsp, err)
		sendDownloadResponse(serverStream, rsp)
		return
	}

	for _, ohlc := range ohlcs {
		if err := ohlcRepo.Store(ctx, asset.Symbol, ohlc.Timestamp.AsTime(),
			ohlc.Open, ohlc.High, ohlc.Low, ohlc.Close, int(ohlc.Volume)); err != nil {
			failDownload(rsp, err)
			sendDownloadResponse(serverStream, rsp)
			continue
		}
	}

	sendDownloadResponse(serverStream, rsp)
}

func (fs *MarketdataServer) DownloadHistoricalData(req *servicePb.DownloadHistoricalDataRequest, serverStream servicePb.Marketdata_DownloadHistoricalDataServer) error {
	ctx := serverStream.Context()

	start := internal_pb.DateToTime(req.GetStartDate())
	var end *time.Time
	if req.GetEndDate() != nil {
		e := internal_pb.DateToTime(req.GetEndDate())
		end = &e
	}

	assetRepo := repository.Asset{Conn: fs.pgx}
	ohlcRepo := repository.OHLC{Conn: fs.pgx}

	var wg sync.WaitGroup
	assetCh := make(chan string, downloadWorkers)
	for range downloadWorkers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for symbol := range assetCh {
				fs.downloadSymbol(ctx, serverStream, symbol, start, end, assetRepo, ohlcRepo)
			}
		}()
	}

	for _, symbol := range req.GetSymbols() {
		assetCh <- symbol
	}
	close(assetCh)
	wg.Wait()
	return nil
}
