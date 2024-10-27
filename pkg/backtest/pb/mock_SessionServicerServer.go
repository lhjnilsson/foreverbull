// Code generated by mockery v2.46.3. DO NOT EDIT.

package pb

import (
	context "context"

	grpc "google.golang.org/grpc"

	mock "github.com/stretchr/testify/mock"
)

// MockSessionServicerServer is an autogenerated mock type for the SessionServicerServer type
type MockSessionServicerServer struct {
	mock.Mock
}

// CreateExecution provides a mock function with given fields: _a0, _a1
func (_m *MockSessionServicerServer) CreateExecution(_a0 context.Context, _a1 *CreateExecutionRequest) (*CreateExecutionResponse, error) {
	ret := _m.Called(_a0, _a1)

	if len(ret) == 0 {
		panic("no return value specified for CreateExecution")
	}

	var r0 *CreateExecutionResponse
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *CreateExecutionRequest) (*CreateExecutionResponse, error)); ok {
		return rf(_a0, _a1)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *CreateExecutionRequest) *CreateExecutionResponse); ok {
		r0 = rf(_a0, _a1)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*CreateExecutionResponse)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *CreateExecutionRequest) error); ok {
		r1 = rf(_a0, _a1)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// RunExecution provides a mock function with given fields: _a0, _a1
func (_m *MockSessionServicerServer) RunExecution(_a0 *RunExecutionRequest, _a1 grpc.ServerStreamingServer[RunExecutionResponse]) error {
	ret := _m.Called(_a0, _a1)

	if len(ret) == 0 {
		panic("no return value specified for RunExecution")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(*RunExecutionRequest, grpc.ServerStreamingServer[RunExecutionResponse]) error); ok {
		r0 = rf(_a0, _a1)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// StopServer provides a mock function with given fields: _a0, _a1
func (_m *MockSessionServicerServer) StopServer(_a0 context.Context, _a1 *StopServerRequest) (*StopServerResponse, error) {
	ret := _m.Called(_a0, _a1)

	if len(ret) == 0 {
		panic("no return value specified for StopServer")
	}

	var r0 *StopServerResponse
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *StopServerRequest) (*StopServerResponse, error)); ok {
		return rf(_a0, _a1)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *StopServerRequest) *StopServerResponse); ok {
		r0 = rf(_a0, _a1)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*StopServerResponse)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *StopServerRequest) error); ok {
		r1 = rf(_a0, _a1)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// mustEmbedUnimplementedSessionServicerServer provides a mock function with given fields:
func (_m *MockSessionServicerServer) mustEmbedUnimplementedSessionServicerServer() {
	_m.Called()
}

// NewMockSessionServicerServer creates a new instance of MockSessionServicerServer. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewMockSessionServicerServer(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockSessionServicerServer {
	mock := &MockSessionServicerServer{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
