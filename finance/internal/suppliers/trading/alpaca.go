package trading

import (
	"github.com/alpacahq/alpaca-trade-api-go/v3/alpaca"
	"github.com/lhjnilsson/foreverbull/internal/config"
)

type AlpacaClient struct {
	client *alpaca.Client
}

func NewAlpacaClient(config *config.Config) (*AlpacaClient, error) {
	client := alpaca.NewClient(alpaca.ClientOpts{
		BaseURL:   config.Provider.Alpaca.BaseURL,
		APIKey:    config.Provider.Alpaca.APIKey,
		APISecret: config.Provider.Alpaca.APISecret,
	})

	return &AlpacaClient{client: client}, nil
}

/*
func (a *AlpacaClient) PlaceOrder(order *storage.Order) error {
	var qty decimal.Decimal
	var side alpaca.Side
	if order.Amount > 0 {
		qty = decimal.NewFromInt(int64(order.Amount))
		side = alpaca.Side("buy")
	} else {
		qty = decimal.NewFromInt(int64(-order.Amount))
		side = alpaca.Side("sell")
	}
	_, err := a.client.PlaceOrder(alpaca.PlaceOrderRequest{
		Symbol:      order.Symbol,
		Qty:         &qty,
		Side:        side,
		Type:        alpaca.Market,
		TimeInForce: alpaca.Day,
	})
	return err
}

func (a *AlpacaClient) GetOrders() ([]*storage.Order, error) {
	orders, err := a.client.GetOrders(alpaca.GetOrdersRequest{})
	if err != nil {
		return nil, err
	}
	var result []*storage.Order
	for _, order := range orders {
		var amount int
		if order.Side == "buy" {
			amount = int(order.Qty.IntPart())
		} else {
			amount = int(-order.Qty.IntPart())
		}
		result = append(result, &storage.Order{
			Symbol: order.Symbol,
			Amount: amount,
		})
	}
	return result, nil
}

func (a *AlpacaClient) GetPositions() ([]*storage.Position, error) {
	positions, err := a.client.GetPositions()
	if err != nil {
		return nil, err
	}
	var result []*storage.Position
	for _, position := range positions {
		result = append(result, &storage.Position{
			Symbol: position.Symbol,
			Amount: int(position.Qty.IntPart()),
		})
	}
	return result, nil
}
*/
