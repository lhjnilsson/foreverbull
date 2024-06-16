package entity

import (
	"github.com/shopspring/decimal"
)

type Position struct {
	Symbol   string `json:"symbol" mapstructure:"symbol"`
	Exchange string `json:"exchange" mapstructure:"exchange"`

	Amount    decimal.Decimal `json:"amount" mapstructure:"amount"`
	CostBasis decimal.Decimal `json:"cost_basis" mapstructure:"cost_basis"`
	Side      string          `json:"side" mapstructure:"side"`
}

/*
type Position struct {
	AssetID                string           `json:"asset_id"`
	Symbol                 string           `json:"symbol"`
	Exchange               string           `json:"exchange"`
	AssetClass             AssetClass       `json:"asset_class"`
	AssetMarginable        bool             `json:"asset_marginable"`
	Qty                    decimal.Decimal  `json:"qty"`
	QtyAvailable           decimal.Decimal  `json:"qty_available"`
	AvgEntryPrice          decimal.Decimal  `json:"avg_entry_price"`
	Side                   string           `json:"side"`
	MarketValue            *decimal.Decimal `json:"market_value"`
	CostBasis              decimal.Decimal  `json:"cost_basis"`
	UnrealizedPL           *decimal.Decimal `json:"unrealized_pl"`
	UnrealizedPLPC         *decimal.Decimal `json:"unrealized_plpc"`
	UnrealizedIntradayPL   *decimal.Decimal `json:"unrealized_intraday_pl"`
	UnrealizedIntradayPLPC *decimal.Decimal `json:"unrealized_intraday_plpc"`
	CurrentPrice           *decimal.Decimal `json:"current_price"`
	LastdayPrice           *decimal.Decimal `json:"lastday_price"`
	ChangeToday            *decimal.Decimal `json:"change_today"`
}
*/
