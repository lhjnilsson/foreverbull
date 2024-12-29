// Code generated by mockery v2.50.1. DO NOT EDIT.

package supplier

import (
	pb "github.com/lhjnilsson/foreverbull/pkg/finance/pb"
	mock "github.com/stretchr/testify/mock"
)

// MockTrading is an autogenerated mock type for the Trading type
type MockTrading struct {
	mock.Mock
}

// GetOrders provides a mock function with no fields
func (_m *MockTrading) GetOrders() ([]*pb.Order, error) {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for GetOrders")
	}

	var r0 []*pb.Order
	var r1 error
	if rf, ok := ret.Get(0).(func() ([]*pb.Order, error)); ok {
		return rf()
	}
	if rf, ok := ret.Get(0).(func() []*pb.Order); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]*pb.Order)
		}
	}

	if rf, ok := ret.Get(1).(func() error); ok {
		r1 = rf()
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetPortfolio provides a mock function with no fields
func (_m *MockTrading) GetPortfolio() (*pb.Portfolio, error) {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for GetPortfolio")
	}

	var r0 *pb.Portfolio
	var r1 error
	if rf, ok := ret.Get(0).(func() (*pb.Portfolio, error)); ok {
		return rf()
	}
	if rf, ok := ret.Get(0).(func() *pb.Portfolio); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*pb.Portfolio)
		}
	}

	if rf, ok := ret.Get(1).(func() error); ok {
		r1 = rf()
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// PlaceOrder provides a mock function with given fields: _a0
func (_m *MockTrading) PlaceOrder(_a0 *pb.Order) (*pb.Order, error) {
	ret := _m.Called(_a0)

	if len(ret) == 0 {
		panic("no return value specified for PlaceOrder")
	}

	var r0 *pb.Order
	var r1 error
	if rf, ok := ret.Get(0).(func(*pb.Order) (*pb.Order, error)); ok {
		return rf(_a0)
	}
	if rf, ok := ret.Get(0).(func(*pb.Order) *pb.Order); ok {
		r0 = rf(_a0)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*pb.Order)
		}
	}

	if rf, ok := ret.Get(1).(func(*pb.Order) error); ok {
		r1 = rf(_a0)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// NewMockTrading creates a new instance of MockTrading. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewMockTrading(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockTrading {
	mock := &MockTrading{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
