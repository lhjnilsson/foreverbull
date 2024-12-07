// Code generated by mockery v2.46.3. DO NOT EDIT.

package pb

import (
	context "context"

	mock "github.com/stretchr/testify/mock"
)

// MockTradingServer is an autogenerated mock type for the TradingServer type
type MockTradingServer struct {
	mock.Mock
}

// GetOrders provides a mock function with given fields: _a0, _a1
func (_m *MockTradingServer) GetOrders(_a0 context.Context, _a1 *GetOrdersRequest) (*GetOrdersResponse, error) {
	ret := _m.Called(_a0, _a1)

	if len(ret) == 0 {
		panic("no return value specified for GetOrders")
	}

	var r0 *GetOrdersResponse
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *GetOrdersRequest) (*GetOrdersResponse, error)); ok {
		return rf(_a0, _a1)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *GetOrdersRequest) *GetOrdersResponse); ok {
		r0 = rf(_a0, _a1)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*GetOrdersResponse)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *GetOrdersRequest) error); ok {
		r1 = rf(_a0, _a1)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetPortfolio provides a mock function with given fields: _a0, _a1
func (_m *MockTradingServer) GetPortfolio(_a0 context.Context, _a1 *GetPortfolioRequest) (*GetPortfolioResponse, error) {
	ret := _m.Called(_a0, _a1)

	if len(ret) == 0 {
		panic("no return value specified for GetPortfolio")
	}

	var r0 *GetPortfolioResponse
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *GetPortfolioRequest) (*GetPortfolioResponse, error)); ok {
		return rf(_a0, _a1)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *GetPortfolioRequest) *GetPortfolioResponse); ok {
		r0 = rf(_a0, _a1)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*GetPortfolioResponse)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *GetPortfolioRequest) error); ok {
		r1 = rf(_a0, _a1)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// PlaceOrder provides a mock function with given fields: _a0, _a1
func (_m *MockTradingServer) PlaceOrder(_a0 context.Context, _a1 *PlaceOrderRequest) (*PlaceOrderResponse, error) {
	ret := _m.Called(_a0, _a1)

	if len(ret) == 0 {
		panic("no return value specified for PlaceOrder")
	}

	var r0 *PlaceOrderResponse
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *PlaceOrderRequest) (*PlaceOrderResponse, error)); ok {
		return rf(_a0, _a1)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *PlaceOrderRequest) *PlaceOrderResponse); ok {
		r0 = rf(_a0, _a1)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*PlaceOrderResponse)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *PlaceOrderRequest) error); ok {
		r1 = rf(_a0, _a1)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// mustEmbedUnimplementedTradingServer provides a mock function with given fields:
func (_m *MockTradingServer) mustEmbedUnimplementedTradingServer() {
	_m.Called()
}

// NewMockTradingServer creates a new instance of MockTradingServer. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewMockTradingServer(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockTradingServer {
	mock := &MockTradingServer{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
