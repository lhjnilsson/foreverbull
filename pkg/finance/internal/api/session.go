package api

import (
	"context"
	"time"

	"github.com/lhjnilsson/foreverbull/pkg/finance/supplier"
	pb "github.com/lhjnilsson/foreverbull/pkg/pb/finance"
	servicePb "github.com/lhjnilsson/foreverbull/pkg/pb/finance/service"
	"github.com/lhjnilsson/foreverbull/pkg/service/worker"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
)

type grpcSessionServer struct {
	servicePb.UnimplementedSessionServicerServer

	trading supplier.Trading

	wp worker.Pool

	orders []*pb.Order

	activity chan bool
}

func NewGRPCSessionServer(trading supplier.Trading) (*grpc.Server, <-chan bool, error) {
	grpcServer := grpc.NewServer()
	activity := make(chan bool)
	server := &grpcSessionServer{
		trading:  trading,
		activity: activity,
	}
	servicePb.RegisterSessionServicerServer(grpcServer, server)

	return grpcServer, activity, nil
}

func (gs *grpcSessionServer) Configure(ctx context.Context, req *servicePb.ConfigureRequest) (*servicePb.ConfigureResponse, error) {
	var err error
	gs.wp, err = worker.NewPool(ctx, req.GetAlgorithm())
	if err != nil {
		return nil, err
	}
	configuration := gs.wp.Configure()
	return &servicePb.ConfigureResponse{
		Configuration: configuration,
	}, nil
}

func (gs *grpcSessionServer) Run(ctx context.Context, req *servicePb.RunRequest) (*servicePb.RunResponse, error) {
	portfolio, err := gs.trading.GetPortfolio()
	if err != nil {
		return nil, err
	}

	orders, err := gs.wp.Process(ctx, time.Now(), req.Symbols, portfolio)
	if err != nil {
		return nil, err
	}
	gs.orders = orders
	return &servicePb.RunResponse{
		Orders: orders,
	}, nil
}

func (gs *grpcSessionServer) PlaceOrders(ctx context.Context, req *servicePb.PlaceOrderRequest) (*servicePb.PlaceOrderResponse, error) {
	rsp := &servicePb.PlaceOrderResponse{
		Orders: make([]*pb.Order, len(req.Orders)),
	}
	for i, order := range req.Orders {
		_, err := gs.trading.PlaceOrder(order)
		if err != nil {
			return nil, err
		}
		rsp.Orders[i] = order
	}
	return rsp, nil
}

func (gs *grpcSessionServer) StopServer(ctx context.Context, req *servicePb.StopServerRequest) (*servicePb.StopServerResponse, error) {
	if gs.wp != nil {
		if err := gs.wp.Close(); err != nil {
			log.Error().Err(err).Msg("error closing worker pool")
		}
	}
	close(gs.activity)
	return &servicePb.StopServerResponse{}, nil
}
