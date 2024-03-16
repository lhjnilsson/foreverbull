package entity

import "time"

type Strategy struct {
	Name    string   `json:"name"`
	Symbols []string `json:"symbols"`
	MinDays int      `json:"min_days"`
	Service *string  `json:"service"`

	CreatedAt time.Time `json:"created_at"`
}
