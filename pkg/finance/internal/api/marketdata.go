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

func (fs *MarketdataServer) GetAsset(ctx context.Context, req *servicePb.GetAssetRequest) (*servicePb.GetAssetResponse, error) {
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

func (fs *MarketdataServer) GetIndex(ctx context.Context, req *servicePb.GetIndexRequest) (*servicePb.GetIndexResponse, error) {
	assets, err := fs.marketdata.GetIndex(req.GetSymbol())
	if err != nil {
		return nil, fmt.Errorf("error getting index: %w", err)
	}

	return &servicePb.GetIndexResponse{
		Assets: assets,
	}, nil
}

func (fs *MarketdataServer) DownloadHistoricalData(req *servicePb.DownloadHistoricalDataRequest, _stream servicePb.Marketdata_DownloadHistoricalDataServer) error {
	ctx := _stream.Context()

	start := internal_pb.DateToTime(req.GetStartDate())
	var end *time.Time
	if req.GetEndDate() != nil {
		e := internal_pb.DateToTime(req.GetEndDate())
		end = &e
	}

	assetRepo := repository.Asset{Conn: fs.pgx}
	ohlcRepo := repository.OHLC{Conn: fs.pgx}

	var wg sync.WaitGroup
	assetCh := make(chan string, 10)
	for range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for symbol := range assetCh {
				rsp := &servicePb.DownloadHistoricalDataResponse{
					Symbol: symbol,
					Status: servicePb.DownloadHistoricalDataResponse_STATUS_OK,
				}
				asset, err := fs.marketdata.GetAsset(symbol)
				if err != nil {
					rsp.Status = servicePb.DownloadHistoricalDataResponse_STATUS_ERROR
					rsp.Error = err.Error()
					if err := _stream.Send(rsp); err != nil {
						log.Err(err).Msg("error sending response")
					}
					continue
				}
				if err := assetRepo.Store(ctx, asset.Symbol, asset.Name); err != nil {
					rsp.Status = servicePb.DownloadHistoricalDataResponse_STATUS_ERROR
					rsp.Error = err.Error()
					if err := _stream.Send(rsp); err != nil {
						log.Err(err).Msg("error sending response")
					}
					continue
				}
				ohlcs, err := fs.marketdata.GetOHLC(asset.Symbol, start, end)
				if err != nil {
					rsp.Status = servicePb.DownloadHistoricalDataResponse_STATUS_ERROR
					rsp.Error = err.Error()
					if err := _stream.Send(rsp); err != nil {
						log.Err(err).Msg("error sending response")
					}
					continue
				}
				for _, ohlc := range ohlcs {
					if err := ohlcRepo.Store(ctx, asset.Symbol, ohlc.Timestamp.AsTime(),
						ohlc.Open, ohlc.High, ohlc.Low, ohlc.Close, int(ohlc.Volume)); err != nil {
						rsp.Status = servicePb.DownloadHistoricalDataResponse_STATUS_ERROR
						rsp.Error = err.Error()
						if err := _stream.Send(rsp); err != nil {
							log.Err(err).Msg("error sending response")
						}
						continue
					}
				}
				if err := _stream.Send(rsp); err != nil {
					log.Err(err).Msg("error sending response")
				}
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
