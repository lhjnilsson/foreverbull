package entity

import (
	"github.com/shopspring/decimal"
)

type Portfolio struct {
	Cash  decimal.Decimal `json:"cash"`
	Value decimal.Decimal `json:"value"`

	Positions []Position `json:"positions"`
}

/*
Alpaca Struct

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

	ALGO:  ['__class__', '__delattr__', '__dict__', '__dir__', '__doc__', '__eq__', '__format__', '__ge__', '__getattribute__',
	'__getstate__', '__gt__', '__hash__', '__init__', '__init_subclass__', '__le__', '__lt__', '__module__', '__ne__', '__new__', '__reduce__',
	'__reduce_ex__', '__repr__', '__setattr__', '__sizeof__', '__str__', '__subclasshook__', '__weakref__',
	'capital_used', 'cash', 'cash_flow', 'current_portfolio_weights', 'pnl', 'portfolio_value',
	'positions', 'positions_exposure', 'positions_value', 'returns', 'start_date', 'starting_cash']

	ALGO:  Portfolio({'cash_flow': -1240.6299999999999, 'starting_cash': 100000, 'portfolio_value': 100049.37000000001,
	'pnl': 49.370000000009895, 'returns': 0.0004937000000000413, 'cash': 98759.37000000001,
	'positions': {Equity(0 [AAPL]): Position({'asset': Equity(0 [AAPL]), 'amount': 10, 'cost_basis': 124.06299999999999, 'last_sale_price': 129.0, 'last_sale_date': Timestamp('2023-01-06 21:00:00+0000', tz='UTC')})},
	'start_date': Timestamp('2023-01-03 00:00:00'), 'positions_value': 1290.0, 'positions_exposure': 1290.0})
}
*/
