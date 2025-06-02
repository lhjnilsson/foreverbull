package api

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/stream"
	"github.com/lhjnilsson/foreverbull/pkg/finance/internal/repository"
	financeStream "github.com/lhjnilsson/foreverbull/pkg/finance/stream"
	"github.com/lhjnilsson/foreverbull/pkg/finance/supplier"
	servicePb "github.com/lhjnilsson/foreverbull/pkg/pb/finance/service"
)

type TradingServer struct {
	servicePb.UnimplementedTradingServicerServer

	pgx    *pgxpool.Pool
	stream stream.Stream

	trading supplier.Trading
}

func NewTradingServer(pgx *pgxpool.Pool, stream stream.Stream, trading supplier.Trading) *TradingServer {
	return &TradingServer{
		pgx:     pgx,
		stream:  stream,
		trading: trading,
	}
}

func (ts *TradingServer) GetOrders(ctx context.Context, req *servicePb.GetOrdersRequest) (*servicePb.GetOrdersResponse, error) {
	orders, err := ts.trading.GetOrders()
	if err != nil {
		return nil, fmt.Errorf("error getting orders: %w", err)
	}
	rsp := &servicePb.GetOrdersResponse{
		Orders: orders,
	}
	return rsp, nil
}

func (ts *TradingServer) GetPortfolio(ctx context.Context, req *servicePb.GetPortfolioRequest) (*servicePb.GetPortfolioResponse, error) {
	portfolio, err := ts.trading.GetPortfolio()
	if err != nil {
		return nil, fmt.Errorf("error getting portfolio: %w", err)
	}
	rsp := &servicePb.GetPortfolioResponse{
		Portfolio: portfolio,
	}
	return rsp, nil
}

func (ts *TradingServer) CreateSession(req *servicePb.CreateSessionRequest, stream servicePb.TradingServicer_CreateSessionServer) error {
	repository := repository.Trading{Conn: ts.pgx}
	s, err := repository.Create(stream.Context())
	if err != nil {
		return fmt.Errorf("error creating session: %w", err)
	}

	o, err := financeStream.NewRunTradeOrchestration(s.Id, req.Symbols)
	if err != nil {
		return err
	}
	ch := ts.stream.RunOrchestration(stream.Context(), o)
	for msg := range ch {
		err = stream.Send(&servicePb.CreateSessionResponse{
			Status: &servicePb.CreateSessionResponse_Status{
				Status: servicePb.CreateSessionResponse_SessionStatus(msg.Status),
				State:  msg.State,
			},
		})
		if err != nil {
			return err
		}
	}
	return nil
}
