package api

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/lhjnilsson/foreverbull/internal/auth"
	"github.com/lhjnilsson/foreverbull/internal/usercopy"
	"github.com/lhjnilsson/foreverbull/pkg/finance/internal/suppliers/marketdata"
	"github.com/lhjnilsson/foreverbull/pkg/finance/supplier"
	pb "github.com/lhjnilsson/foreverbull/pkg/pb/finance"
	servicePb "github.com/lhjnilsson/foreverbull/pkg/pb/finance/service"
	"github.com/lhjnilsson/foreverbull/pkg/service/worker"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type grpcSessionServer struct {
	servicePb.UnimplementedSessionServicerServer

	trading supplier.Trading

	wp worker.Pool

	orders []*pb.Order

	activity chan bool

	bars     worker.BarSource
	provider string

	// sessionID names the trading session in the user's sentences.
	sessionID string
}

type sessionOption func(*grpcSessionServer)

// WithBars fills WorkerRequest.bars from source during paper runs.
// provider is the price source. Configure refuses an empty or unknown one.
func WithBars(source worker.BarSource, provider string) sessionOption {
	return func(gs *grpcSessionServer) {
		gs.bars = source
		gs.provider = provider
	}
}

// WithSessionID names the trading session the server runs.
func WithSessionID(id string) sessionOption {
	return func(gs *grpcSessionServer) {
		gs.sessionID = id
	}
}

func NewGRPCSessionServer(trading supplier.Trading, opts ...sessionOption) (*grpc.Server, <-chan bool, error) {
	grpcServer := grpc.NewServer()
	activity := make(chan bool)
	server := &grpcSessionServer{
		trading:  trading,
		activity: activity,
	}
	for _, opt := range opts {
		opt(server)
	}
	servicePb.RegisterSessionServicerServer(grpcServer, server)

	return grpcServer, activity, nil
}

// NewSecuredGRPCSessionServer requires x-foreverbull-session on every call.
func NewSecuredGRPCSessionServer(trading supplier.Trading, secret string, opts ...sessionOption) (*grpc.Server, <-chan bool, error) {
	if secret == "" {
		return nil, nil, fmt.Errorf("%s is missing", auth.SessionMetadata())
	}
	grpcServer := grpc.NewServer(auth.SessionInterceptors(secret)...)
	activity := make(chan bool)
	server := &grpcSessionServer{
		trading:  trading,
		activity: activity,
	}
	for _, opt := range opts {
		opt(server)
	}
	servicePb.RegisterSessionServicerServer(grpcServer, server)
	return grpcServer, activity, nil
}

func (gs *grpcSessionServer) Configure(ctx context.Context, req *servicePb.ConfigureRequest) (*servicePb.ConfigureResponse, error) {
	var poolOpts []worker.Option
	if gs.bars != nil {
		provider, err := marketdata.RequireTradingPrice(gs.provider)
		if err != nil {
			return nil, status.Error(codes.FailedPrecondition, err.Error())
		}
		poolOpts = append(poolOpts, worker.WithBars(gs.bars, provider))
	}
	var err error
	gs.wp, err = worker.NewPool(ctx, req.GetAlgorithm(), poolOpts...)
	if err != nil {
		return nil, err
	}
	configuration := gs.wp.Configure()
	return &servicePb.ConfigureResponse{
		Configuration: configuration,
	}, nil
}

func (gs *grpcSessionServer) Run(ctx context.Context, req *servicePb.RunRequest) (*servicePb.RunResponse, error) {
	portfolio, err := gs.trading.GetPortfolio(ctx)
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

// PlaceOrders attempts every order. A placed order is the order PlaceOrder
// returned. A failure is that request order with status REJECTED and
// reject_reason set to the cause. Any failure fails the call. The response
// still lists the orders that placed and every rejection, and the status
// carries the same response: a unary RPC does not send the reply when the
// handler returns an error.
func (gs *grpcSessionServer) PlaceOrders(ctx context.Context, req *servicePb.PlaceOrderRequest) (*servicePb.PlaceOrderResponse, error) {
	rsp := &servicePb.PlaceOrderResponse{
		Orders: make([]*pb.Order, len(req.GetOrders())),
	}
	var failed []error
	var placed []string
	for i, order := range req.GetOrders() {
		filled, err := gs.trading.PlaceOrder(ctx, order)
		if err == nil && filled == nil {
			err = fmt.Errorf("place order returned no order: %w", supplier.ErrUnavailable)
		}
		if err != nil {
			rsp.Orders[i] = rejectedOrder(order, err)
			failed = append(failed, fmt.Errorf("%s: %w", orderCauseLabel(order, i), err))
			continue
		}
		rsp.Orders[i] = filled
		placed = append(placed, placedOrderNote(filled, i))
	}
	if len(failed) == 0 {
		return rsp, nil
	}
	parts := make([]error, 0, 1+len(placed)+len(failed))
	parts = append(parts, errors.New("place orders failed"))
	for _, note := range placed {
		parts = append(parts, errors.New("placed "+note))
	}
	parts = append(parts, failed...)
	return rsp, &placeOrdersError{
		orders: rsp,
		err:    errors.Join(parts...),
		code:   placeOrdersCode(ctx, failed),
	}
}

// placeOrdersError is a PlaceOrders call that did not place every order.
// orders is the full request: placed orders and rejected ones with causes.
type placeOrdersError struct {
	orders *servicePb.PlaceOrderResponse
	err    error
	code   codes.Code
}

func (e *placeOrdersError) Error() string {
	if e == nil || e.err == nil {
		return "place orders failed"
	}
	return e.err.Error()
}

func (e *placeOrdersError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.err
}

// GRPCStatus puts the order list on the status. The unary handler's reply
// is not delivered when the call fails.
func (e *placeOrdersError) GRPCStatus() *status.Status {
	st := status.New(e.code, e.Error())
	if e.orders == nil {
		return st
	}
	with, err := st.WithDetails(e.orders)
	if err != nil {
		return st
	}
	return with
}

func placeOrdersCode(ctx context.Context, failed []error) codes.Code {
	if err := ctx.Err(); err != nil {
		return status.FromContextError(err).Code()
	}
	for _, err := range failed {
		if errors.Is(err, supplier.ErrUnavailable) {
			return codes.Unavailable
		}
	}
	return codes.FailedPrecondition
}

func rejectedOrder(order *pb.Order, cause error) *pb.Order {
	out := &pb.Order{
		Status:       pb.OrderStatus_ORDER_STATUS_REJECTED,
		RejectReason: cause.Error(),
	}
	if order == nil {
		return out
	}
	out.Symbol = order.GetSymbol()
	out.Isin = order.GetIsin()
	out.Amount = order.GetAmount()
	out.Type = order.GetType()
	out.TimeInForce = order.GetTimeInForce()
	out.LimitPrice = order.GetLimitPrice()
	return out
}

func orderCauseLabel(order *pb.Order, index int) string {
	ident := fmt.Sprintf("order %d", index+1)
	if order == nil {
		return ident
	}
	symbol := strings.TrimSpace(order.GetSymbol())
	isin := strings.TrimSpace(order.GetIsin())
	switch {
	case symbol != "" && isin != "":
		ident = fmt.Sprintf("order %d %s %s", index+1, symbol, isin)
	case symbol != "":
		ident = fmt.Sprintf("order %d %s", index+1, symbol)
	case isin != "":
		ident = fmt.Sprintf("order %d %s", index+1, isin)
	}
	return fmt.Sprintf("%s amount %d", ident, order.GetAmount())
}

func placedOrderNote(order *pb.Order, index int) string {
	note := orderCauseLabel(order, index)
	if order == nil {
		return note
	}
	if id := strings.TrimSpace(order.GetBrokerOrderId()); id != "" {
		return note + " broker " + id
	}
	return note
}

func (gs *grpcSessionServer) StopServer(ctx context.Context, req *servicePb.StopServerRequest) (*servicePb.StopServerResponse, error) {
	var closeErr error
	if gs.wp != nil {
		closeErr = gs.wp.Close()
	}
	// The session server stops either way; a pool that didn't close is
	// still the caller's failure.
	close(gs.activity)
	if closeErr != nil {
		// The socket error names local addresses, so it goes to the log.
		log.Error().Err(closeErr).Str("session", gs.sessionID).Msg("error closing worker pool")
		return nil, status.Error(codes.Internal, usercopy.TradingSessionStopFailed(gs.sessionID))
	}
	return &servicePb.StopServerResponse{}, nil
}
