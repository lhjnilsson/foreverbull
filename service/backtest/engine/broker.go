package engine

import (
	"fmt"

	finance "github.com/lhjnilsson/foreverbull/finance/entity"
	"github.com/lhjnilsson/foreverbull/service/message"
)

type Broker interface {
	Order(*finance.Order) (*finance.Order, error)
	GetOrder(*finance.Order) (*finance.Order, error)
	CancelOrder(order *finance.Order) error
}

/*
Order
places a new order, can be positive or negative value for long or short
*/
func (b *Zipline) Order(order *finance.Order) (*finance.Order, error) {
	ctxSock, err := b.socket.Get()
	if err != nil {
		return nil, fmt.Errorf("error opening context socket: %w", err)
	}
	defer ctxSock.Close()
	req := message.Request{Task: "order", Data: order}

	rsp, err := req.Process(ctxSock)
	if err != nil {
		return nil, fmt.Errorf("error processing order: %w", err)
	}
	if err = rsp.DecodeData(order); err != nil {
		return nil, fmt.Errorf("error decoding order- data: %w", err)
	}
	return order, nil
}

/*
GetOrder
Get information about an order, if its filled or not
*/
func (b *Zipline) GetOrder(order *finance.Order) (*finance.Order, error) {
	ctxSock, err := b.socket.Get()
	if err != nil {
		return nil, fmt.Errorf("error opening context socket: %w", err)
	}
	defer ctxSock.Close()
	req := message.Request{Task: "get_order", Data: order}

	rsp, err := req.Process(ctxSock)
	if err != nil {
		return nil, fmt.Errorf("error processing order: %w", err)
	}
	newOrder := finance.Order{}
	if err = rsp.DecodeData(&newOrder); err != nil {
		return nil, fmt.Errorf("error decoding order- data: %w", err)
	}
	return &newOrder, nil
}

/*
CancelOrder
Cancels an order being placed
*/
func (b *Zipline) CancelOrder(order *finance.Order) error {
	ctxSock, err := b.socket.Get()
	if err != nil {
		return fmt.Errorf("error opening context socket: %w", err)
	}
	defer ctxSock.Close()

	req := message.Request{Task: "cancel_order", Data: order}
	rsp, err := req.Process(ctxSock)
	if err != nil {
		return fmt.Errorf("error processing order: %w", err)
	}
	if err = rsp.DecodeData(order); err != nil {
		return fmt.Errorf("error decoding order- data: %w", err)
	}
	return nil
}
