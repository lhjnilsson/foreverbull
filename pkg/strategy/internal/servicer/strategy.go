package servicer

import (
	"fmt"
	"time"

	finance_pb "github.com/lhjnilsson/foreverbull/pkg/pb/finance"
	pb "github.com/lhjnilsson/foreverbull/pkg/pb/strategy"
	"github.com/lhjnilsson/foreverbull/pkg/service/worker"
	"github.com/rs/zerolog/log"
)

type StrategyServer struct {
	pb.UnimplementedStrategyServicerServer

	marketdata finance_pb.MarketdataClient
}

func NewStrategyServer(md finance_pb.MarketdataClient) *StrategyServer {
	return &StrategyServer{
		marketdata: md,
	}
}

func (ss *StrategyServer) RunStrategy(req *pb.RunStrategyRequest, stream pb.StrategyServicer_RunStrategyServer) error {
	ctx := stream.Context()

	err := stream.Send(&pb.RunStrategyResponse{
		Status: &pb.RunStrategyResponse_Status{
			Status: pb.RunStrategyResponse_Status_UPDATING_MARKETDATA,
		},
	})
	if err != nil {
		return fmt.Errorf("error sending response: %w", err)
	}
	_, err = ss.marketdata.DownloadHistoricalData(ctx, &finance_pb.DownloadHistoricalDataRequest{
		Symbols:   req.GetSymbols(),
		StartDate: req.GetStartDate(),
	})
	if err != nil {
		errStr := fmt.Sprintf("error downloading historical data: %v", err)
		if sErr := stream.Send(&pb.RunStrategyResponse{
			Status: &pb.RunStrategyResponse_Status{
				Status: pb.RunStrategyResponse_Status_FAILED,
				Error:  &errStr,
			},
		}); sErr != nil {
			return fmt.Errorf("error sending response: %w", sErr)
		}
		return fmt.Errorf("error downloading historical data: %w", err)
	}

	err = stream.Send(&pb.RunStrategyResponse{
		Status: &pb.RunStrategyResponse_Status{
			Status: pb.RunStrategyResponse_Status_CREATING_WORKER_POOL,
		},
	})
	if err != nil {
		return fmt.Errorf("error sending response: %w", err)
	}
	wp, err := worker.NewPool(ctx, req.GetAlgorithm())
	if err != nil {
		errStr := fmt.Sprintf("error creating worker pool: %v", err)
		if sErr := stream.Send(&pb.RunStrategyResponse{
			Status: &pb.RunStrategyResponse_Status{
				Status: pb.RunStrategyResponse_Status_FAILED,
				Error:  &errStr,
			},
		}); err != nil {
			return fmt.Errorf("error sending response: %w", sErr)
		}
		return fmt.Errorf("error creating worker pool: %w", err)
	}

	configuration := wp.Configure()
	err = stream.Send(&pb.RunStrategyResponse{
		Status: &pb.RunStrategyResponse_Status{
			Status: pb.RunStrategyResponse_Status_READY,
		},
		Configuration: configuration,
	})
	if err != nil {
		return fmt.Errorf("error sending response: %w", err)
	}

	orders, err := wp.Process(ctx, time.Now(), []string{}, nil)
	if err != nil {
		errStr := fmt.Sprintf("error creating worker pool: %v", err)
		if sErr := stream.Send(&pb.RunStrategyResponse{
			Status: &pb.RunStrategyResponse_Status{
				Status: pb.RunStrategyResponse_Status_FAILED,
				Error:  &errStr,
			},
		}); err != nil {
			return fmt.Errorf("error sending response: %w", sErr)
		}
		return fmt.Errorf("error creating worker pool: %w", err)
	}
	log.Info().Msgf("ORDERS: %v", orders)

	err = stream.Send(&pb.RunStrategyResponse{
		Status: &pb.RunStrategyResponse_Status{
			Status: pb.RunStrategyResponse_Status_RUNNING,
		},
	})
	if err != nil {
		return fmt.Errorf("error sending response: %w", err)
	}

	err = stream.Send(&pb.RunStrategyResponse{
		Status: &pb.RunStrategyResponse_Status{
			Status: pb.RunStrategyResponse_Status_COMPLETED,
		},
	})
	if err != nil {
		return fmt.Errorf("error sending response: %w", err)
	}
	return nil
}
