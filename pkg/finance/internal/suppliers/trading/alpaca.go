package trading

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/alpacahq/alpaca-trade-api-go/v3/alpaca"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/pkg/finance/supplier"
	pb "github.com/lhjnilsson/foreverbull/pkg/pb/finance"
	"github.com/shopspring/decimal"
	"google.golang.org/protobuf/types/known/timestamppb"
)

var _ supplier.Trading = (*AlpacaClient)(nil)

type AlpacaClient struct {
	client *alpaca.Client
}

func NewAlpacaClient() (*AlpacaClient, error) {
	baseUrl, found := os.LookupEnv(environment.AlpacaBaseURL)
	if !found {
		return nil, fmt.Errorf("failed to find environment variable %s", environment.AlpacaBaseURL)
	}
	apiKey, found := os.LookupEnv(environment.AlpacaApiKey)
	if !found {
		return nil, fmt.Errorf("failed to find environment variable %s", environment.AlpacaApiKey)
	}
	apiSecret, found := os.LookupEnv(environment.AlpacaApiSecret)
	if !found {
		return nil, fmt.Errorf("failed to find environment variable %s", environment.AlpacaApiSecret)
	}

	client := alpaca.NewClient(alpaca.ClientOpts{
		BaseURL:   baseUrl,
		APIKey:    apiKey,
		APISecret: apiSecret,
	})

	acc, err := client.GetAccount()
	if err != nil {
		return nil, fmt.Errorf("failed to get account: %w", err)
	}

	if acc.AccountBlocked {
		return nil, errors.New("account is blocked")
	}

	if acc.TradingBlocked {
		return nil, errors.New("trading is blocked")
	}

	return &AlpacaClient{client: client}, nil
}

func (c *AlpacaClient) GetPortfolio(ctx context.Context) (*pb.Portfolio, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	acc, err := c.client.GetAccount()
	if err != nil {
		return nil, fmt.Errorf("failed to get account: %w", err)
	}

	pos, err := c.client.GetPositions()
	if err != nil {
		return nil, fmt.Errorf("failed to get positions: %w", err)
	}

	var positions []*pb.Position
	for _, position := range pos {
		positions = append(positions, &pb.Position{
			Symbol:    position.Symbol,
			Amount:    int32((position.Qty.InexactFloat64())),
			CostBasis: position.CostBasis.InexactFloat64(),
		})
	}

	return &pb.Portfolio{
		Timestamp:      timestamppb.Now(),
		Cash:           acc.Cash.InexactFloat64(),
		PortfolioValue: acc.PortfolioValue.InexactFloat64(),
		Positions:      positions,
	}, nil
}

func (c *AlpacaClient) PlaceOrder(ctx context.Context, order *pb.Order) (*pb.Order, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	qty := decimal.NewFromInt32(order.Amount)
	var side alpaca.Side
	if qty.IsNegative() {
		side = alpaca.Sell
	} else {
		side = alpaca.Buy
	}
	qty = qty.Abs()

	var orderType alpaca.OrderType
	switch order.Type {
	case pb.OrderType_ORDER_TYPE_MARKET:
		orderType = alpaca.Market
	case pb.OrderType_ORDER_TYPE_LIMIT:
		orderType = alpaca.Limit
	case pb.OrderType_ORDER_TYPE_UNSPECIFIED:
		return nil, fmt.Errorf("unknown order type: %s", order.Type)
	default:
		return nil, fmt.Errorf("unknown order type: %s", order.Type)
	}

	var timeInForce alpaca.TimeInForce
	switch order.TimeInForce {
	case pb.TimeInForce_TIME_IN_FORCE_DAY:
		timeInForce = alpaca.Day
	case pb.TimeInForce_TIME_IN_FORCE_UNSPECIFIED:
		return nil, fmt.Errorf("unknown time in force: %s", order.TimeInForce)
	default:
		return nil, fmt.Errorf("unknown time in force: %s", order.TimeInForce)
	}

	var intent alpaca.PositionIntent
	switch order.Intent {
	case pb.PositionIntent_POSITION_INTENT_BUY_TO_OPEN:
		intent = alpaca.BuyToOpen
	case pb.PositionIntent_POSITION_INTENT_SELL_TO_CLOSE:
		intent = alpaca.SellToClose
	case pb.PositionIntent_POSITION_INTENT_BUY_TO_CLOSE:
		intent = alpaca.BuyToClose
	case pb.PositionIntent_POSITION_INTENT_SELL_TO_OPEN:
		intent = alpaca.SellToOpen
	case pb.PositionIntent_POSITION_INTENT_UNSPECIFIED:
		return nil, fmt.Errorf("unknown position intent: %s", order.Intent)
	default:
		return nil, fmt.Errorf("unknown position intent: %s", order.Intent)
	}

	req := alpaca.PlaceOrderRequest{
		Symbol:         order.Symbol,
		Qty:            &qty,
		Side:           side,
		Type:           orderType,
		TimeInForce:    timeInForce,
		PositionIntent: intent,
	}
	_, err := c.client.PlaceOrder(req)
	if err != nil {
		return nil, fmt.Errorf("failed to place order: %w", err)
	}
	return order, nil
}

func (c *AlpacaClient) GetOrders(ctx context.Context) ([]*pb.Order, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	_, err := c.client.GetOrders(alpaca.GetOrdersRequest{})
	if err != nil {
		return nil, fmt.Errorf("failed to get orders: %w", err)
	}

	var entityOrders []*pb.Order

	return entityOrders, nil
}
