package servicer

import (
	"fmt"

	"github.com/lhjnilsson/foreverbull/pkg/service/worker"
	"github.com/lhjnilsson/foreverbull/pkg/strategy/pb"
)

type StrategyServer struct {
	pb.UnimplementedStrategyServicerServer
}

func NewStrategyServer() *StrategyServer {
	return &StrategyServer{}
}

func (ss *StrategyServer) RunStrategy(req *pb.RunStrategyRequest, stream pb.StrategyServicer_RunStrategyServer) error {
	ctx := stream.Context()
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
