package entity

import "time"

type Portfolio struct {
	Cash      float64    `json:"cash"`
	Value     float64    `json:"value"`
	Positions []Position `json:"positions"`
}

type Position struct {
	Symbol    *string    `json:"symbol" mapstructure:"symbol"`
	Amount    *int       `json:"amount" mapstructure:"amount"`
	CostBasis *float64   `json:"cost_basis" mapstructure:"cost_basis"`
	Period    *time.Time `json:"period" mapstructure:"period"`
}
