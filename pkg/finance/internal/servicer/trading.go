package servicer

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/pkg/finance/pb"
	"github.com/lhjnilsson/foreverbull/pkg/finance/supplier"
)

type TradingServer struct {
	pb.UnimplementedTradingServer

	pgx     *pgxpool.Pool
	trading supplier.Trading
}

func NewTradingServer(pgx *pgxpool.Pool, trading supplier.Trading) *TradingServer {
	return &TradingServer{
		pgx:     pgx,
		trading: trading,
	}
}

func (ts *TradingServer) GetOrders(ctx context.Context, req *pb.GetOrdersRequest) (*pb.GetOrdersResponse, error) {
	orders, err := ts.trading.GetOrders()
	if err != nil {
		return nil, fmt.Errorf("error getting orders: %w", err)
	}
	rsp := &pb.GetOrdersResponse{
		Orders: orders,
	}
	return rsp, nil
}

func (ts *TradingServer) PlaceOrder(ctx context.Context, req *pb.PlaceOrderRequest) (*pb.PlaceOrderResponse, error) {
	_, err := ts.trading.PlaceOrder(req.Order)
	if err != nil {
		return nil, fmt.Errorf("error placing order: %w", err)
	}
	return &pb.PlaceOrderResponse{}, nil
}

func (ts *TradingServer) GetPortfolio(ctx context.Context, req *pb.GetPortfolioRequest) (*pb.GetPortfolioResponse, error) {
	portfolio, err := ts.trading.GetPortfolio()
	if err != nil {
		return nil, fmt.Errorf("error getting portfolio: %w", err)
	}
	rsp := &pb.GetPortfolioResponse{
		Portfolio: portfolio,
	}
	return rsp, nil
}
