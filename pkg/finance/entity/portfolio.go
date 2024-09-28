package entity

import (
	"github.com/shopspring/decimal"
)

type Portfolio struct {
	Cash  decimal.Decimal `json:"cash" mapstructure:"cash"`
	Value decimal.Decimal `json:"value" mapstructure:"value"`

	Positions []Position `json:"positions" mapstructure:"positions"`
}
