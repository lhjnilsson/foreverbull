package api

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/stream"
	"github.com/lhjnilsson/foreverbull/pkg/finance/internal/repository"
	financeStream "github.com/lhjnilsson/foreverbull/pkg/finance/stream"
	"github.com/lhjnilsson/foreverbull/pkg/finance/supplier"
	commonPb "github.com/lhjnilsson/foreverbull/pkg/pb"
	servicePb "github.com/lhjnilsson/foreverbull/pkg/pb/finance/service"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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
	if ts.trading == nil {
		return nil, fmt.Errorf("trading is not enabled")
	}

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
	if ts.trading == nil {
		return nil, fmt.Errorf("trading is not enabled")
	}

	portfolio, err := ts.trading.GetPortfolio()
	if err != nil {
		return nil, fmt.Errorf("error getting portfolio: %w", err)
	}
	rsp := &servicePb.GetPortfolioResponse{
		Portfolio: portfolio,
	}
	return rsp, nil
}

func (ts *TradingServer) CreateSession(req *servicePb.CreateSessionRequest, _stream servicePb.TradingServicer_CreateSessionServer) error {
	if ts.trading == nil {
		return status.Error(codes.Unimplemented, "trading is not enabled")
	}
	rsp := &servicePb.CreateSessionResponse{
		CREATING: &commonPb.Step{
			State: commonPb.State_STATE_RUNNING,
		},
		UPDATING_MARKET_DATA: &commonPb.Step{
			State: commonPb.State_STATE_UNSPECIFIED,
		},
		STARTING: &commonPb.Step{
			State: commonPb.State_STATE_UNSPECIFIED,
		},
		FINISHING: &commonPb.Step{
			State: commonPb.State_STATE_UNSPECIFIED,
		},
		Session: nil,
	}
	if err := _stream.Send(rsp); err != nil {
		return fmt.Errorf("error sending response: %w", err)
	}

	repository := repository.Trading{Conn: ts.pgx}
	s, err := repository.Create(_stream.Context())
	if err != nil {
		return fmt.Errorf("error creating session: %w", err)
	}
	rsp.Session = s
	rsp.CREATING.State = commonPb.State_STATE_COMPLETED
	if err := _stream.Send(rsp); err != nil {
		return fmt.Errorf("error sending response: %w", err)
	}
	o, err := financeStream.NewRunTradeOrchestration(s.Id, req.Symbols)
	if err != nil {
		return err
	}
	ch := ts.stream.RunOrchestration(_stream.Context(), o)
	for msg := range ch {
		if err := stream.UpdateOrchestration(rsp, msg); err != nil {
			return fmt.Errorf("error updating orchestration: %w", err)
		}
		rsp.Session, err = repository.Get(_stream.Context(), s.Id)
		if err != nil {
			return err
		}
		if err := _stream.Send(rsp); err != nil {
			return fmt.Errorf("error sending response: %w", err)
		}
	}
	return nil
}
