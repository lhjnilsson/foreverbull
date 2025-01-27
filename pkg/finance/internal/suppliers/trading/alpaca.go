package trading

import (
	"errors"
	"fmt"

	"github.com/alpacahq/alpaca-trade-api-go/v3/alpaca"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	pb "github.com/lhjnilsson/foreverbull/pkg/pb/finance"
	"github.com/shopspring/decimal"
)

type AlpacaClient struct {
	client *alpaca.Client
}

func NewAlpacaClient() (*AlpacaClient, error) {
	client := alpaca.NewClient(alpaca.ClientOpts{
		BaseURL:   environment.GetAlpacaBaseURL(),
		APIKey:    environment.GetAlpacaAPIKey(),
		APISecret: environment.GetAlpacaAPISecret(),
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

func (c *AlpacaClient) GetPortfolio() (*pb.Portfolio, error) {
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
		Cash:           acc.Cash.InexactFloat64(),
		PortfolioValue: acc.PortfolioValue.InexactFloat64(),
		Positions:      positions,
	}, nil
}

func (c *AlpacaClient) PlaceOrder(order *pb.Order) (*pb.Order, error) {
	qty := decimal.NewFromInt32(order.Amount)
	var side alpaca.Side
	if qty.IsNegative() {
		side = alpaca.Sell
	} else {
		side = alpaca.Buy
	}

	_, err := c.client.PlaceOrder(alpaca.PlaceOrderRequest{
		Symbol: order.Symbol,
		Qty:    &qty,
		Side:   side,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to place order: %w", err)
	}

	return order, nil
}

func (c *AlpacaClient) GetOrders() ([]*pb.Order, error) {
	_, err := c.client.GetOrders(alpaca.GetOrdersRequest{})
	if err != nil {
		return nil, fmt.Errorf("failed to get orders: %w", err)
	}

	var entityOrders []*pb.Order

	return entityOrders, nil
}
