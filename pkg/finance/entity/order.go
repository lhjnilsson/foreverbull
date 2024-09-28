package entity

type Order struct {
	Symbol string `json:"symbol" mapstructure:"symbol"`
	Amount int32  `json:"amount" mapstructure:"amount"`
}
