package entity

import (
	"time"
)

type Strategy struct {
	Name      *string    `json:"name"`
	Backtest  *string    `json:"backtest"`
	Schedule  *string    `json:"schedule"`
	CreatedAt *time.Time `json:"created_at"`
}
