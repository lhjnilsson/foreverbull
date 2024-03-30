package trading

import (
	"fmt"

	"github.com/alpacahq/alpaca-trade-api-go/v3/alpaca"
	"github.com/lhjnilsson/foreverbull/finance/entity"
	"github.com/lhjnilsson/foreverbull/internal/environment"
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
		return nil, fmt.Errorf("account is blocked")
	}
	if acc.TradingBlocked {
		return nil, fmt.Errorf("trading is blocked")
	}
	return &AlpacaClient{client: client}, nil
}

func (c *AlpacaClient) GetPortfolio() (*entity.Portfolio, error) {
	acc, err := c.client.GetAccount()
	if err != nil {
		return nil, fmt.Errorf("failed to get account: %w", err)
	}
	pos, err := c.client.GetPositions()
	if err != nil {
		return nil, fmt.Errorf("failed to get positions: %w", err)
	}
	var positions []entity.Position
	for _, position := range pos {
		positions = append(positions, entity.Position{
			Symbol:    position.Symbol,
			Exchange:  position.Exchange,
			Amount:    position.Qty,
			CostBasis: position.CostBasis,
			Side:      position.Side,
		})
	}
	return &entity.Portfolio{
		Cash:           acc.Cash,
		PortfolioValue: acc.PortfolioValue,
		Positions:      positions,
	}, nil
}

func (c *AlpacaClient) GetOrders() ([]*entity.Order, error) {
	orders, err := c.client.GetOrders(alpaca.GetOrdersRequest{})
	if err != nil {
		return nil, fmt.Errorf("failed to get orders: %w", err)
	}
	var entityOrders []*entity.Order
	for _, order := range orders {
		entityOrders = append(entityOrders, &entity.Order{
			ID:          order.ID,
			CreatedAt:   order.CreatedAt,
			UpdatedAt:   order.UpdatedAt,
			SubmittedAt: order.SubmittedAt,
			FilledAt:    order.FilledAt,
			ExpiredAt:   order.ExpiredAt,
			CanceledAt:  order.CanceledAt,
			FailedAt:    order.FailedAt,
			ReplacedAt:  order.ReplacedAt,
			Symbol:      order.Symbol,
			Side:        string(order.Side),
		})
	}
	return entityOrders, nil
}
