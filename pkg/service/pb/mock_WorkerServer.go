// Code generated by mockery v2.46.3. DO NOT EDIT.

package pb

import (
	context "context"

	mock "github.com/stretchr/testify/mock"
)

// MockWorkerServer is an autogenerated mock type for the WorkerServer type
type MockWorkerServer struct {
	mock.Mock
}

// ConfigureExecution provides a mock function with given fields: _a0, _a1
func (_m *MockWorkerServer) ConfigureExecution(_a0 context.Context, _a1 *ConfigureExecutionRequest) (*ConfigureExecutionResponse, error) {
	ret := _m.Called(_a0, _a1)

	if len(ret) == 0 {
		panic("no return value specified for ConfigureExecution")
	}

	var r0 *ConfigureExecutionResponse
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *ConfigureExecutionRequest) (*ConfigureExecutionResponse, error)); ok {
		return rf(_a0, _a1)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *ConfigureExecutionRequest) *ConfigureExecutionResponse); ok {
		r0 = rf(_a0, _a1)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*ConfigureExecutionResponse)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *ConfigureExecutionRequest) error); ok {
		r1 = rf(_a0, _a1)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetServiceInfo provides a mock function with given fields: _a0, _a1
func (_m *MockWorkerServer) GetServiceInfo(_a0 context.Context, _a1 *GetServiceInfoRequest) (*GetServiceInfoResponse, error) {
	ret := _m.Called(_a0, _a1)

	if len(ret) == 0 {
		panic("no return value specified for GetServiceInfo")
	}

	var r0 *GetServiceInfoResponse
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *GetServiceInfoRequest) (*GetServiceInfoResponse, error)); ok {
		return rf(_a0, _a1)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *GetServiceInfoRequest) *GetServiceInfoResponse); ok {
		r0 = rf(_a0, _a1)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*GetServiceInfoResponse)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *GetServiceInfoRequest) error); ok {
		r1 = rf(_a0, _a1)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// RunExecution provides a mock function with given fields: _a0, _a1
func (_m *MockWorkerServer) RunExecution(_a0 context.Context, _a1 *RunExecutionRequest) (*RunExecutionResponse, error) {
	ret := _m.Called(_a0, _a1)

	if len(ret) == 0 {
		panic("no return value specified for RunExecution")
	}

	var r0 *RunExecutionResponse
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *RunExecutionRequest) (*RunExecutionResponse, error)); ok {
		return rf(_a0, _a1)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *RunExecutionRequest) *RunExecutionResponse); ok {
		r0 = rf(_a0, _a1)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*RunExecutionResponse)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *RunExecutionRequest) error); ok {
		r1 = rf(_a0, _a1)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// mustEmbedUnimplementedWorkerServer provides a mock function with given fields:
func (_m *MockWorkerServer) mustEmbedUnimplementedWorkerServer() {
	_m.Called()
}

// NewMockWorkerServer creates a new instance of MockWorkerServer. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewMockWorkerServer(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockWorkerServer {
	mock := &MockWorkerServer{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
