package entity

import (
	"time"
)

/*
Asset
Contains details about a single asset
*/
type Asset struct {
	Symbol      string     `json:"symbol" mapstructure:"symbol"`
	Name        string     `json:"name" mapstructure:"name"`
	Title       string     `json:"title" mapstructure:"title"`
	Type        string     `json:"type" mapstructure:"type"`
	LastUpdated time.Time  `json:"last_updated" mapstructure:"last_updated"`
	StartOHLC   *time.Time `json:"start_ohlc" mapstructure:"start_ohlc"`
	EndOHLC     *time.Time `json:"end_ohlc" mapstructure:"end_ohlc"`
}
