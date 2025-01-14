// Code generated by mockery v2.50.1. DO NOT EDIT.

package worker

import (
	context "context"

	financepb "github.com/lhjnilsson/foreverbull/pkg/finance/pb"
	mock "github.com/stretchr/testify/mock"

	pb "github.com/lhjnilsson/foreverbull/pkg/service/pb"

	time "time"
)

// MockPool is an autogenerated mock type for the Pool type
type MockPool struct {
	mock.Mock
}

// Close provides a mock function with no fields
func (_m *MockPool) Close() error {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for Close")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func() error); ok {
		r0 = rf()
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// Configure provides a mock function with no fields
func (_m *MockPool) Configure() *pb.ExecutionConfiguration {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for Configure")
	}

	var r0 *pb.ExecutionConfiguration
	if rf, ok := ret.Get(0).(func() *pb.ExecutionConfiguration); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*pb.ExecutionConfiguration)
		}
	}

	return r0
}

// Process provides a mock function with given fields: ctx, timestamp, symbols, portfolio
func (_m *MockPool) Process(ctx context.Context, timestamp time.Time, symbols []string, portfolio *financepb.Portfolio) ([]*financepb.Order, error) {
	ret := _m.Called(ctx, timestamp, symbols, portfolio)

	if len(ret) == 0 {
		panic("no return value specified for Process")
	}

	var r0 []*financepb.Order
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, time.Time, []string, *financepb.Portfolio) ([]*financepb.Order, error)); ok {
		return rf(ctx, timestamp, symbols, portfolio)
	}
	if rf, ok := ret.Get(0).(func(context.Context, time.Time, []string, *financepb.Portfolio) []*financepb.Order); ok {
		r0 = rf(ctx, timestamp, symbols, portfolio)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]*financepb.Order)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, time.Time, []string, *financepb.Portfolio) error); ok {
		r1 = rf(ctx, timestamp, symbols, portfolio)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// NewMockPool creates a new instance of MockPool. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewMockPool(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockPool {
	mock := &MockPool{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
