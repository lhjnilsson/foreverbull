package api

import "github.com/lhjnilsson/foreverbull/pkg/strategy/entity"

type CreateStrategyBody struct {
	Name    string   `json:"name" binding:"required,gte=3,lte=32"`
	Symbols []string `json:"symbols" binding:"required,gte=1"`
	MinDays int      `json:"min_days" binding:"required,gte=1"`
	Service string   `json:"service"`
}

type CreateStrategyResponse entity.Strategy
type ListStrategyResponse []entity.Strategy
type GetStrategyResponse entity.Strategy

type CreateExecutionBody struct {
	Strategy string `json:"strategy" binding:"required"`
}
type CreateExecutionResponse entity.Execution
type ListExecutionResponse []entity.Execution
type GetExecutionResponse entity.Execution
