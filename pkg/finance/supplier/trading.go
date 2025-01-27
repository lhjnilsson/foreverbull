package supplier

import (
	pb "github.com/lhjnilsson/foreverbull/pkg/pb/finance"
)

type Trading interface {
	GetPortfolio() (*pb.Portfolio, error)
	PlaceOrder(*pb.Order) (*pb.Order, error)
	GetOrders() ([]*pb.Order, error)
}
