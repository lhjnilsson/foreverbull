package stream

import (
	"context"

	"github.com/stretchr/testify/mock"
)

type MockExperimentLauncher struct {
	mock.Mock
}

func (_m *MockExperimentLauncher) Run(ctx context.Context, in RunInput) (*RunOutput, error) {
	ret := _m.Called(ctx, in)

	if len(ret) == 0 {
		panic("no return value specified for Run")
	}

	var r0 *RunOutput
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, RunInput) (*RunOutput, error)); ok {
		return rf(ctx, in)
	}
	if rf, ok := ret.Get(0).(func(context.Context, RunInput) *RunOutput); ok {
		r0 = rf(ctx, in)
	} else if ret.Get(0) != nil {
		r0 = ret.Get(0).(*RunOutput)
	}

	if rf, ok := ret.Get(1).(func(context.Context, RunInput) error); ok {
		r1 = rf(ctx, in)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}
