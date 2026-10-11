package supplier

import (
	"context"

	pb "github.com/lhjnilsson/foreverbull/pkg/pb/finance"
)

// Trading is the only broker API a strategy can see.
type Trading interface {
	GetPortfolio(ctx context.Context) (*pb.Portfolio, error)
	PlaceOrder(ctx context.Context, order *pb.Order) (*pb.Order, error)
	GetOrders(ctx context.Context) ([]*pb.Order, error)
}

// Registry resolves a trading profile name to its client.
// Two lookups of the same name return the same client.
type Registry interface {
	Get(name string) (Trading, bool)
}
