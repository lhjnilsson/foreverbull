package entity

import (
	"time"

	"github.com/shopspring/decimal"
)

type Side string

const (
	Buy  Side = "buy"
	Sell Side = "sell"
)

type Order struct {
	ID string `json:"id"`

	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	SubmittedAt time.Time  `json:"submitted_at"`
	FilledAt    *time.Time `json:"filled_at"`
	ExpiredAt   *time.Time `json:"expired_at"`
	CanceledAt  *time.Time `json:"canceled_at"`
	FailedAt    *time.Time `json:"failed_at"`
	ReplacedAt  *time.Time `json:"replaced_at"`

	Symbol string `json:"symbol"`
	Side   string `json:"side"`

	Amount   *decimal.Decimal `json:"amount"`
	Notional *decimal.Decimal `json:"notional"`

	Filled         decimal.Decimal `json:"filled"`
	FilledAvgPrice decimal.Decimal `json:"filled_avg_price"`
}

/*


type Side string

const (
	Buy  Side = "buy"
	Sell Side = "sell"
)

type OrderType string

const (
	Market       OrderType = "market"
	Limit        OrderType = "limit"
	Stop         OrderType = "stop"
	StopLimit    OrderType = "stop_limit"
	TrailingStop OrderType = "trailing_stop"
)

type OrderClass string

const (
	Bracket OrderClass = "bracket"
	OTO     OrderClass = "oto"
	OCO     OrderClass = "oco"
	Simple  OrderClass = "simple"
)

type Order struct {
	ID             string           `json:"id"`
	ClientOrderID  string           `json:"client_order_id"`
	CreatedAt      time.Time        `json:"created_at"`
	UpdatedAt      time.Time        `json:"updated_at"`
	SubmittedAt    time.Time        `json:"submitted_at"`
	FilledAt       *time.Time       `json:"filled_at"`
	ExpiredAt      *time.Time       `json:"expired_at"`
	CanceledAt     *time.Time       `json:"canceled_at"`
	FailedAt       *time.Time       `json:"failed_at"`
	ReplacedAt     *time.Time       `json:"replaced_at"`
	ReplacedBy     *string          `json:"replaced_by"`
	Replaces       *string          `json:"replaces"`
	AssetID        string           `json:"asset_id"`
	Symbol         string           `json:"symbol"`
	AssetClass     AssetClass       `json:"asset_class"`
	OrderClass     OrderClass       `json:"order_class"`
	Type           OrderType        `json:"type"`
	Side           Side             `json:"side"`
	TimeInForce    TimeInForce      `json:"time_in_force"`
	Status         string           `json:"status"`
	Notional       *decimal.Decimal `json:"notional"`
	Qty            *decimal.Decimal `json:"qty"`
	FilledQty      decimal.Decimal  `json:"filled_qty"`
	FilledAvgPrice *decimal.Decimal `json:"filled_avg_price"`
	LimitPrice     *decimal.Decimal `json:"limit_price"`
	StopPrice      *decimal.Decimal `json:"stop_price"`
	TrailPrice     *decimal.Decimal `json:"trail_price"`
	TrailPercent   *decimal.Decimal `json:"trail_percent"`
	HWM            *decimal.Decimal `json:"hwm"`
	ExtendedHours  bool             `json:"extended_hours"`
	Legs           []Order          `json:"legs"`
}

*/
