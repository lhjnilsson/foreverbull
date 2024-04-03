// Code generated by mockery v2.33.2. DO NOT EDIT.

package mocks

import (
	context "context"

	backtest "github.com/lhjnilsson/foreverbull/service/backtest"

	mock "github.com/stretchr/testify/mock"
)

// Backtest is an autogenerated mock type for the Backtest type
type Backtest struct {
	mock.Mock
}

// CancelOrder provides a mock function with given fields: order
func (_m *Backtest) CancelOrder(order *backtest.Order) error {
	ret := _m.Called(order)

	var r0 error
	if rf, ok := ret.Get(0).(func(*backtest.Order) error); ok {
		r0 = rf(order)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// ConfigureExecution provides a mock function with given fields: _a0, _a1
func (_m *Backtest) ConfigureExecution(_a0 context.Context, _a1 *backtest.BacktestConfig) error {
	ret := _m.Called(_a0, _a1)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *backtest.BacktestConfig) error); ok {
		r0 = rf(_a0, _a1)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// Continue provides a mock function with given fields:
func (_m *Backtest) Continue() error {
	ret := _m.Called()

	var r0 error
	if rf, ok := ret.Get(0).(func() error); ok {
		r0 = rf()
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// DownloadIngestion provides a mock function with given fields: _a0, _a1
func (_m *Backtest) DownloadIngestion(_a0 context.Context, _a1 string) error {
	ret := _m.Called(_a0, _a1)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string) error); ok {
		r0 = rf(_a0, _a1)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// GetExecutionResult provides a mock function with given fields: execution
func (_m *Backtest) GetExecutionResult(execution *backtest.Execution) (*backtest.Result, error) {
	ret := _m.Called(execution)

	var r0 *backtest.Result
	var r1 error
	if rf, ok := ret.Get(0).(func(*backtest.Execution) (*backtest.Result, error)); ok {
		return rf(execution)
	}
	if rf, ok := ret.Get(0).(func(*backtest.Execution) *backtest.Result); ok {
		r0 = rf(execution)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*backtest.Result)
		}
	}

	if rf, ok := ret.Get(1).(func(*backtest.Execution) error); ok {
		r1 = rf(execution)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetMessage provides a mock function with given fields:
func (_m *Backtest) GetMessage() (*backtest.Period, error) {
	ret := _m.Called()

	var r0 *backtest.Period
	var r1 error
	if rf, ok := ret.Get(0).(func() (*backtest.Period, error)); ok {
		return rf()
	}
	if rf, ok := ret.Get(0).(func() *backtest.Period); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*backtest.Period)
		}
	}

	if rf, ok := ret.Get(1).(func() error); ok {
		r1 = rf()
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetOrder provides a mock function with given fields: _a0
func (_m *Backtest) GetOrder(_a0 *backtest.Order) (*backtest.Order, error) {
	ret := _m.Called(_a0)

	var r0 *backtest.Order
	var r1 error
	if rf, ok := ret.Get(0).(func(*backtest.Order) (*backtest.Order, error)); ok {
		return rf(_a0)
	}
	if rf, ok := ret.Get(0).(func(*backtest.Order) *backtest.Order); ok {
		r0 = rf(_a0)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*backtest.Order)
		}
	}

	if rf, ok := ret.Get(1).(func(*backtest.Order) error); ok {
		r1 = rf(_a0)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// Ingest provides a mock function with given fields: _a0, _a1
func (_m *Backtest) Ingest(_a0 context.Context, _a1 *backtest.IngestConfig) error {
	ret := _m.Called(_a0, _a1)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *backtest.IngestConfig) error); ok {
		r0 = rf(_a0, _a1)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// Order provides a mock function with given fields: _a0
func (_m *Backtest) Order(_a0 *backtest.Order) (*backtest.Order, error) {
	ret := _m.Called(_a0)

	var r0 *backtest.Order
	var r1 error
	if rf, ok := ret.Get(0).(func(*backtest.Order) (*backtest.Order, error)); ok {
		return rf(_a0)
	}
	if rf, ok := ret.Get(0).(func(*backtest.Order) *backtest.Order); ok {
		r0 = rf(_a0)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*backtest.Order)
		}
	}

	if rf, ok := ret.Get(1).(func(*backtest.Order) error); ok {
		r1 = rf(_a0)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// RunExecution provides a mock function with given fields: _a0
func (_m *Backtest) RunExecution(_a0 context.Context) error {
	ret := _m.Called(_a0)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context) error); ok {
		r0 = rf(_a0)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// Stop provides a mock function with given fields: _a0
func (_m *Backtest) Stop(_a0 context.Context) error {
	ret := _m.Called(_a0)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context) error); ok {
		r0 = rf(_a0)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// UploadIngestion provides a mock function with given fields: _a0, _a1
func (_m *Backtest) UploadIngestion(_a0 context.Context, _a1 string) error {
	ret := _m.Called(_a0, _a1)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string) error); ok {
		r0 = rf(_a0, _a1)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// NewBacktest creates a new instance of Backtest. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewBacktest(t interface {
	mock.TestingT
	Cleanup(func())
}) *Backtest {
	mock := &Backtest{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
