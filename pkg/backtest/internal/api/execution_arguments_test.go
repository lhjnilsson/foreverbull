package api

import (
	"testing"

	"github.com/lhjnilsson/foreverbull/pkg/pb/backtest"
	workerpb "github.com/lhjnilsson/foreverbull/pkg/pb/service"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

func TestExecutionFunctionsFromAlgorithmUsesSuppliedValueOtherwiseDefault(t *testing.T) {
	algo := &workerpb.Algorithm{
		FilePath:   "/tmp/algo.py",
		Namespaces: []string{"signals"},
		Functions: []*workerpb.Algorithm_Function{
			{
				Name: "rebalance",
				Parameters: []*workerpb.Algorithm_FunctionParameter{
					{Key: "fastperiod", Value: strp("12"), DefaultValue: strp("10"), ValueType: "int"},
					{Key: "slowperiod", DefaultValue: strp("26"), ValueType: "int"},
					{Key: "signalperiod", ValueType: "int"},
					{Key: "oversold", Value: strp(""), DefaultValue: strp("30"), ValueType: "int"},
				},
			},
			{Name: "screen"},
			nil,
		},
	}

	got := executionFunctionsFromAlgorithm(algo)
	want := []*backtest.ExecutionFunction{
		{
			Name: "rebalance",
			Parameters: []*backtest.ExecutionFunction_Parameter{
				{Key: "fastperiod", Value: "12", ValueType: "int"},
				{Key: "slowperiod", Value: "26", ValueType: "int"},
				{Key: "signalperiod", Value: "", ValueType: "int"},
				{Key: "oversold", Value: "", ValueType: "int"},
			},
		},
		{Name: "screen", Parameters: []*backtest.ExecutionFunction_Parameter{}},
	}
	require.Len(t, got, len(want))
	for i := range want {
		require.True(t, proto.Equal(want[i], got[i]))
	}

	empty := executionFunctionsFromAlgorithm(nil)
	require.NotNil(t, empty)
	require.Empty(t, empty)
}

func strp(v string) *string {
	return &v
}
