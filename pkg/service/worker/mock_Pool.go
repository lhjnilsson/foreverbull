// Code generated by mockery v2.18.0. DO NOT EDIT.

package worker

import (
	context "context"

	entity "github.com/lhjnilsson/foreverbull/pkg/finance/entity"
	mock "github.com/stretchr/testify/mock"

	serviceentity "github.com/lhjnilsson/foreverbull/pkg/service/entity"

	time "time"
)

// MockPool is an autogenerated mock type for the Pool type
type MockPool struct {
	mock.Mock
}

// Close provides a mock function with given fields:
func (_m *MockPool) Close() error {
	ret := _m.Called()

	var r0 error
	if rf, ok := ret.Get(0).(func() error); ok {
		r0 = rf()
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// GetNamespacePort provides a mock function with given fields:
func (_m *MockPool) GetNamespacePort() int {
	ret := _m.Called()

	var r0 int
	if rf, ok := ret.Get(0).(func() int); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(int)
	}

	return r0
}

// GetPort provides a mock function with given fields:
func (_m *MockPool) GetPort() int {
	ret := _m.Called()

	var r0 int
	if rf, ok := ret.Get(0).(func() int); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(int)
	}

	return r0
}

// Process provides a mock function with given fields: ctx, timestamp, symbols, portfolio
func (_m *MockPool) Process(ctx context.Context, timestamp time.Time, symbols []string, portfolio *entity.Portfolio) (*[]entity.Order, error) {
	ret := _m.Called(ctx, timestamp, symbols, portfolio)

	var r0 *[]entity.Order
	if rf, ok := ret.Get(0).(func(context.Context, time.Time, []string, *entity.Portfolio) *[]entity.Order); ok {
		r0 = rf(ctx, timestamp, symbols, portfolio)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*[]entity.Order)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, time.Time, []string, *entity.Portfolio) error); ok {
		r1 = rf(ctx, timestamp, symbols, portfolio)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// SetAlgorithm provides a mock function with given fields: algo
func (_m *MockPool) SetAlgorithm(algo *serviceentity.Algorithm) error {
	ret := _m.Called(algo)

	var r0 error
	if rf, ok := ret.Get(0).(func(*serviceentity.Algorithm) error); ok {
		r0 = rf(algo)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

type mockConstructorTestingTNewMockPool interface {
	mock.TestingT
	Cleanup(func())
}

// NewMockPool creates a new instance of MockPool. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewMockPool(t mockConstructorTestingTNewMockPool) *MockPool {
	mock := &MockPool{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}