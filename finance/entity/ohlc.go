package entity

import (
	"time"
)

/*
OHLC
End-of-day information of a stock in a simulation
*/
type OHLC struct {
	Time   time.Time `json:"time"`
	Open   float64   `json:"open"`
	High   float64   `json:"high"`
	Low    float64   `json:"low"`
	Close  float64   `json:"close"`
	Volume int       `json:"volume"`
}
