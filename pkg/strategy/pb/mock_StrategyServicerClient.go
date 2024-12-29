// Code generated by mockery v2.50.1. DO NOT EDIT.

package pb

import (
	context "context"

	grpc "google.golang.org/grpc"

	mock "github.com/stretchr/testify/mock"
)

// MockStrategyServicerClient is an autogenerated mock type for the StrategyServicerClient type
type MockStrategyServicerClient struct {
	mock.Mock
}

// RunStrategy provides a mock function with given fields: ctx, in, opts
func (_m *MockStrategyServicerClient) RunStrategy(ctx context.Context, in *RunStrategyRequest, opts ...grpc.CallOption) (grpc.ServerStreamingClient[RunStrategyResponse], error) {
	_va := make([]interface{}, len(opts))
	for _i := range opts {
		_va[_i] = opts[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, ctx, in)
	_ca = append(_ca, _va...)
	ret := _m.Called(_ca...)

	if len(ret) == 0 {
		panic("no return value specified for RunStrategy")
	}

	var r0 grpc.ServerStreamingClient[RunStrategyResponse]
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *RunStrategyRequest, ...grpc.CallOption) (grpc.ServerStreamingClient[RunStrategyResponse], error)); ok {
		return rf(ctx, in, opts...)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *RunStrategyRequest, ...grpc.CallOption) grpc.ServerStreamingClient[RunStrategyResponse]); ok {
		r0 = rf(ctx, in, opts...)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(grpc.ServerStreamingClient[RunStrategyResponse])
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *RunStrategyRequest, ...grpc.CallOption) error); ok {
		r1 = rf(ctx, in, opts...)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// NewMockStrategyServicerClient creates a new instance of MockStrategyServicerClient. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewMockStrategyServicerClient(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockStrategyServicerClient {
	mock := &MockStrategyServicerClient{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
