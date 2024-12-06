package servicer

import (
	"context"

	"github.com/lhjnilsson/foreverbull/pkg/strategy/pb"
)

type StrategyServer struct {
	pb.UnimplementedStrategyServicerServer
}

func NewStrategyServer() *StrategyServer {
	return &StrategyServer{}
}

func (ss *StrategyServer) RunStrategy(ctx context.Context, req *pb.RunStrategyRequest) (*pb.RunStrategyResponse, error) {
	rsp := &pb.RunStrategyResponse{}
	return rsp, nil
}
