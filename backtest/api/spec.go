package api

import (
	"errors"
	"time"

	"github.com/lhjnilsson/foreverbull/backtest/entity"
)

func ParseTime(s string) (time.Time, error) {
	t, err := time.Parse("2006-01-02", s)
	if err == nil {
		return t, nil
	}
	t, err = time.Parse("2006-01-02T15:04:05Z", s)
	if err == nil {
		return t, nil
	}
	return time.Now(), errors.New("invalid time")
}

type CreateBacktestBody struct {
	Name    string  `json:"name" binding:"required,gte=3,lte=32"`
	Service *string `json:"service"`

	Start     string   `json:"start" binding:"required"`
	End       string   `json:"end" binding:"required"`
	Calendar  string   `json:"calendar" binding:"required"`
	Symbols   []string `json:"symbols" binding:"required,gte=1"`
	Benchmark *string  `json:"benchmark"`
}

type CreateBacktestResponse entity.Backtest
type ListBacktestResponse []entity.Backtest
type GetBacktestResponse entity.Backtest

type CreateSessionBody struct {
	Backtest   string `json:"backtest" binding:"required"`
	Manual     bool   `json:"manual"`
	Executions []struct {
		Start     *string   `json:"start"`
		End       *string   `json:"end"`
		Symbols   *[]string `json:"symbols"`
		Benchmark *string   `json:"benchmark"`
	} `json:"executions"`
}
