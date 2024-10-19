package servicer

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	internal_pb "github.com/lhjnilsson/foreverbull/internal/pb"
	"github.com/lhjnilsson/foreverbull/pkg/finance/internal/repository"
	"github.com/lhjnilsson/foreverbull/pkg/finance/pb"
	"github.com/lhjnilsson/foreverbull/pkg/finance/supplier"
	"golang.org/x/sync/errgroup"
)

type FinanceServer struct {
	pb.UnimplementedFinanceServer

	pgx        *pgxpool.Pool
	marketdata supplier.Marketdata
}

func NewFinanceServer(pgx *pgxpool.Pool, md supplier.Marketdata) *FinanceServer {
	return &FinanceServer{
		pgx:        pgx,
		marketdata: md,
	}
}

func (fs *FinanceServer) GetAsset(ctx context.Context, req *pb.GetAssetRequest) (*pb.GetAssetResponse, error) {
	asset, err := fs.marketdata.GetAsset(req.GetSymbol())
	if err != nil {
		return nil, fmt.Errorf("error getting asset: %w", err)
	}

	return &pb.GetAssetResponse{
		Asset: asset,
	}, nil
}

func (fs *FinanceServer) GetIndex(ctx context.Context, req *pb.GetIndexRequest) (*pb.GetIndexResponse, error) {
	assets, err := fs.marketdata.GetIndex(req.GetSymbol())
	if err != nil {
		return nil, fmt.Errorf("error getting index: %w", err)
	}

	return &pb.GetIndexResponse{
		Assets: assets,
	}, nil
}

func (fs *FinanceServer) DownloadHistoricalData(ctx context.Context, req *pb.DownloadHistoricalDataRequest) (*pb.DownloadHistoricalDataResponse, error) {
	assets, err := fs.marketdata.GetIndex(req.GetSymbol())
	if err != nil {
		return nil, fmt.Errorf("error getting index: %w", err)
	}

	start := internal_pb.DateToTime(req.GetStartDate())

	var end *time.Time

	if req.GetEndDate() != nil {
		e := internal_pb.DateToTime(req.GetEndDate())
		end = &e
	}

	asset_repo := repository.Asset{Conn: fs.pgx}
	ohlc_repo := repository.OHLC{Conn: fs.pgx}

	g, gctx := errgroup.WithContext(ctx)

	for _, asset := range assets {
		a := asset

		g.Go(func() error {
			ohlcs, err := fs.marketdata.GetOHLC(a.Symbol, start, end)
			if err != nil {
				return fmt.Errorf("error getting ohlc: %w", err)
			}

			if err := asset_repo.Store(gctx, a.Symbol, a.Name); err != nil {
				return fmt.Errorf("error creating asset: %w", err)
			}

			for _, ohlc := range ohlcs {
				if err := ohlc_repo.Store(gctx, a.Symbol, ohlc.Timestamp.AsTime(),
					ohlc.Open, ohlc.High, ohlc.Low, ohlc.Close, int(ohlc.Volume)); err != nil {
					return fmt.Errorf("error creating ohlc: %w", err)
				}
			}

			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, fmt.Errorf("error downloading historical data: %w", err)
	}

	return &pb.DownloadHistoricalDataResponse{}, nil
}
