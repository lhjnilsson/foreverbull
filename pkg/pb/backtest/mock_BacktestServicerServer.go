// Code generated by mockery v2.46.3. DO NOT EDIT.

package backtest

import (
	context "context"

	mock "github.com/stretchr/testify/mock"
)

// MockBacktestServicerServer is an autogenerated mock type for the BacktestServicerServer type
type MockBacktestServicerServer struct {
	mock.Mock
}

// CreateBacktest provides a mock function with given fields: _a0, _a1
func (_m *MockBacktestServicerServer) CreateBacktest(_a0 context.Context, _a1 *CreateBacktestRequest) (*CreateBacktestResponse, error) {
	ret := _m.Called(_a0, _a1)

	if len(ret) == 0 {
		panic("no return value specified for CreateBacktest")
	}

	var r0 *CreateBacktestResponse
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *CreateBacktestRequest) (*CreateBacktestResponse, error)); ok {
		return rf(_a0, _a1)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *CreateBacktestRequest) *CreateBacktestResponse); ok {
		r0 = rf(_a0, _a1)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*CreateBacktestResponse)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *CreateBacktestRequest) error); ok {
		r1 = rf(_a0, _a1)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// CreateSession provides a mock function with given fields: _a0, _a1
func (_m *MockBacktestServicerServer) CreateSession(_a0 context.Context, _a1 *CreateSessionRequest) (*CreateSessionResponse, error) {
	ret := _m.Called(_a0, _a1)

	if len(ret) == 0 {
		panic("no return value specified for CreateSession")
	}

	var r0 *CreateSessionResponse
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *CreateSessionRequest) (*CreateSessionResponse, error)); ok {
		return rf(_a0, _a1)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *CreateSessionRequest) *CreateSessionResponse); ok {
		r0 = rf(_a0, _a1)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*CreateSessionResponse)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *CreateSessionRequest) error); ok {
		r1 = rf(_a0, _a1)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetBacktest provides a mock function with given fields: _a0, _a1
func (_m *MockBacktestServicerServer) GetBacktest(_a0 context.Context, _a1 *GetBacktestRequest) (*GetBacktestResponse, error) {
	ret := _m.Called(_a0, _a1)

	if len(ret) == 0 {
		panic("no return value specified for GetBacktest")
	}

	var r0 *GetBacktestResponse
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *GetBacktestRequest) (*GetBacktestResponse, error)); ok {
		return rf(_a0, _a1)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *GetBacktestRequest) *GetBacktestResponse); ok {
		r0 = rf(_a0, _a1)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*GetBacktestResponse)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *GetBacktestRequest) error); ok {
		r1 = rf(_a0, _a1)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetExecution provides a mock function with given fields: _a0, _a1
func (_m *MockBacktestServicerServer) GetExecution(_a0 context.Context, _a1 *GetExecutionRequest) (*GetExecutionResponse, error) {
	ret := _m.Called(_a0, _a1)

	if len(ret) == 0 {
		panic("no return value specified for GetExecution")
	}

	var r0 *GetExecutionResponse
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *GetExecutionRequest) (*GetExecutionResponse, error)); ok {
		return rf(_a0, _a1)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *GetExecutionRequest) *GetExecutionResponse); ok {
		r0 = rf(_a0, _a1)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*GetExecutionResponse)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *GetExecutionRequest) error); ok {
		r1 = rf(_a0, _a1)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetSession provides a mock function with given fields: _a0, _a1
func (_m *MockBacktestServicerServer) GetSession(_a0 context.Context, _a1 *GetSessionRequest) (*GetSessionResponse, error) {
	ret := _m.Called(_a0, _a1)

	if len(ret) == 0 {
		panic("no return value specified for GetSession")
	}

	var r0 *GetSessionResponse
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *GetSessionRequest) (*GetSessionResponse, error)); ok {
		return rf(_a0, _a1)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *GetSessionRequest) *GetSessionResponse); ok {
		r0 = rf(_a0, _a1)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*GetSessionResponse)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *GetSessionRequest) error); ok {
		r1 = rf(_a0, _a1)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ListBacktests provides a mock function with given fields: _a0, _a1
func (_m *MockBacktestServicerServer) ListBacktests(_a0 context.Context, _a1 *ListBacktestsRequest) (*ListBacktestsResponse, error) {
	ret := _m.Called(_a0, _a1)

	if len(ret) == 0 {
		panic("no return value specified for ListBacktests")
	}

	var r0 *ListBacktestsResponse
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *ListBacktestsRequest) (*ListBacktestsResponse, error)); ok {
		return rf(_a0, _a1)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *ListBacktestsRequest) *ListBacktestsResponse); ok {
		r0 = rf(_a0, _a1)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*ListBacktestsResponse)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *ListBacktestsRequest) error); ok {
		r1 = rf(_a0, _a1)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ListExecutions provides a mock function with given fields: _a0, _a1
func (_m *MockBacktestServicerServer) ListExecutions(_a0 context.Context, _a1 *ListExecutionsRequest) (*ListExecutionsResponse, error) {
	ret := _m.Called(_a0, _a1)

	if len(ret) == 0 {
		panic("no return value specified for ListExecutions")
	}

	var r0 *ListExecutionsResponse
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *ListExecutionsRequest) (*ListExecutionsResponse, error)); ok {
		return rf(_a0, _a1)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *ListExecutionsRequest) *ListExecutionsResponse); ok {
		r0 = rf(_a0, _a1)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*ListExecutionsResponse)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *ListExecutionsRequest) error); ok {
		r1 = rf(_a0, _a1)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// mustEmbedUnimplementedBacktestServicerServer provides a mock function with given fields:
func (_m *MockBacktestServicerServer) mustEmbedUnimplementedBacktestServicerServer() {
	_m.Called()
}

// NewMockBacktestServicerServer creates a new instance of MockBacktestServicerServer. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewMockBacktestServicerServer(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockBacktestServicerServer {
	mock := &MockBacktestServicerServer{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
