package supplier

import "github.com/lhjnilsson/foreverbull/pkg/finance/pb"

type Trading interface {
	GetPortfolio() (*pb.Portfolio, error)
	PlaceOrder(*pb.Order) (*pb.Order, error)
	GetOrders() ([]*pb.Order, error)
}
