package entity

import "time"

type Portfolio struct {
	Cash      float64    `json:"cash"`
	Value     float64    `json:"value"`
	Positions []Position `json:"positions"`
}

/*
type Account struct {
	ID                    string          `json:"id"`
	AccountNumber         string          `json:"account_number"`
	Status                string          `json:"status"`
	CryptoStatus          string          `json:"crypto_status"`
	Currency              string          `json:"currency"`
	BuyingPower           decimal.Decimal `json:"buying_power"`
	RegTBuyingPower       decimal.Decimal `json:"regt_buying_power"`
	DaytradingBuyingPower decimal.Decimal `json:"daytrading_buying_power"`
	EffectiveBuyingPower  decimal.Decimal `json:"effective_buying_power"`
	NonMarginBuyingPower  decimal.Decimal `json:"non_marginable_buying_power"`
	BodDtbp               decimal.Decimal `json:"bod_dtbp"`
	Cash                  decimal.Decimal `json:"cash"`
	AccruedFees           decimal.Decimal `json:"accrued_fees"`
	PortfolioValue        decimal.Decimal `json:"portfolio_value"`
	PatternDayTrader      bool            `json:"pattern_day_trader"`
	TradingBlocked        bool            `json:"trading_blocked"`
	TransfersBlocked      bool            `json:"transfers_blocked"`
	AccountBlocked        bool            `json:"account_blocked"`
	ShortingEnabled       bool            `json:"shorting_enabled"`
	TradeSuspendedByUser  bool            `json:"trade_suspended_by_user"`
	CreatedAt             time.Time       `json:"created_at"`
	Multiplier            decimal.Decimal `json:"multiplier"`
	Equity                decimal.Decimal `json:"equity"`
	LastEquity            decimal.Decimal `json:"last_equity"`
	LongMarketValue       decimal.Decimal `json:"long_market_value"`
	ShortMarketValue      decimal.Decimal `json:"short_market_value"`
	PositionMarketValue   decimal.Decimal `json:"position_market_value"`
	InitialMargin         decimal.Decimal `json:"initial_margin"`
	MaintenanceMargin     decimal.Decimal `json:"maintenance_margin"`
	LastMaintenanceMargin decimal.Decimal `json:"last_maintenance_margin"`
	SMA                   decimal.Decimal `json:"sma"`
	DaytradeCount         int64           `json:"daytrade_count"`
	CryptoTier            int             `json:"crypto_tier"`
}
*/

type Position struct {
	Symbol    *string    `json:"symbol" mapstructure:"symbol"`
	Amount    *int       `json:"amount" mapstructure:"amount"`
	CostBasis *float64   `json:"cost_basis" mapstructure:"cost_basis"`
	Period    *time.Time `json:"period" mapstructure:"period"`
	Side      *string    `json:"side" mapstructure:"side"`
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
