package api

import (
	pb "github.com/lhjnilsson/foreverbull/pkg/pb/strategy/service"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// StrategyServer is a gRPC stub. RunStrategy is unimplemented and is not a
// live-trading entrypoint; strategy.Module is not registered on the broker.
type StrategyServer struct {
	pb.UnimplementedStrategyServicerServer
}

func NewStrategyServer() *StrategyServer {
	return &StrategyServer{}
}

func (*StrategyServer) RunStrategy(_ *pb.RunStrategyRequest, _ pb.StrategyServicer_RunStrategyServer) error {
	return status.Error(codes.Unimplemented, "RunStrategy is not implemented")
}
