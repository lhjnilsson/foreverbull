package entity

import (
	"time"
)

type OrderStatus int

const (
	OPEN      OrderStatus = iota
	FILLED    OrderStatus = iota
	CANCELLED OrderStatus = iota
	REJECTED  OrderStatus = iota
	HELD      OrderStatus = iota
)

/*
Order
Stock order data
*/
type Order struct {
	ID         *string      `json:"id" mapstructure:"id"`
	Execution  *string      `json:"execution" mapstructure:"execution"`
	Symbol     *string      `json:"symbol" mapstructure:"symbol"`
	Amount     *int         `json:"amount" mapstructure:"amount"`
	LimitPrice *int         `json:"limit_price" mapstructure:"limit_price"`
	StopPrice  *int         `json:"stop_price" mapstructure:"stop_price"`
	Filled     *int         `json:"filled" mapstructure:"filled"`
	Commission *int         `json:"commission" mapstructure:"commission"`
	Status     *OrderStatus `json:"status" mapstructure:"status"`
	CreatedAt  *time.Time   `json:"created_at" mapstructure:"created_at"`
}
