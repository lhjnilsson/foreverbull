package entity

import (
	"time"

	"github.com/shopspring/decimal"
)

type Position struct {
	Symbol        string          `json:"symbol" mapstructure:"symbol"`
	Amount        decimal.Decimal `json:"amount" mapstructure:"amount"`
	CostBasis     decimal.Decimal `json:"cost_basis" mapstructure:"cost_basis"`
	LastSalePrice decimal.Decimal `json:"last_price" mapstructure:"last_sale_price"`
	LastSaleDate  time.Time       `json:"last_sale_date" mapstructure:"last_sale_date"`
}
