package api

import (
	pb "github.com/lhjnilsson/foreverbull/pkg/pb/strategy/service"
)

// StrategyServer is a gRPC stub. Lifecycle RPCs are unimplemented until later
// PoC steps; strategy.Module is not registered on the broker.
type StrategyServer struct {
	pb.UnimplementedStrategyServicerServer
}

func NewStrategyServer() *StrategyServer {
	return &StrategyServer{}
}
