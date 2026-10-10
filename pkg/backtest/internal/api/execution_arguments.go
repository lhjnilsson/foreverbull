package api

import (
	"github.com/lhjnilsson/foreverbull/pkg/pb/backtest"
	workerpb "github.com/lhjnilsson/foreverbull/pkg/pb/service"
)

// executionFunctionsFromAlgorithm records the arguments a run actually used.
// A set value wins, including an explicit empty string. Otherwise the default is stored.
// Nothing is invented for parameters the algorithm did not declare.
func executionFunctionsFromAlgorithm(algo *workerpb.Algorithm) []*backtest.ExecutionFunction {
	out := make([]*backtest.ExecutionFunction, 0)
	if algo == nil {
		return out
	}

	for _, fn := range algo.GetFunctions() {
		if fn == nil {
			continue
		}
		params := make([]*backtest.ExecutionFunction_Parameter, 0, len(fn.GetParameters()))
		for _, p := range fn.GetParameters() {
			if p == nil {
				continue
			}
			params = append(params, &backtest.ExecutionFunction_Parameter{
				Key:       p.GetKey(),
				Value:     usedParameterValue(p),
				ValueType: p.GetValueType(),
			})
		}
		out = append(out, &backtest.ExecutionFunction{
			Name:       fn.GetName(),
			Parameters: params,
		})
	}

	return out
}

func usedParameterValue(p *workerpb.Algorithm_FunctionParameter) string {
	if p == nil {
		return ""
	}
	if p.Value != nil {
		return p.GetValue()
	}
	return p.GetDefaultValue()
}
