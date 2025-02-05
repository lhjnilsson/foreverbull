// Code generated by mockery v2.46.3. DO NOT EDIT.

package backtest

import (
	context "context"

	grpc "google.golang.org/grpc"

	mock "github.com/stretchr/testify/mock"
)

// MockIngestionServicerServer is an autogenerated mock type for the IngestionServicerServer type
type MockIngestionServicerServer struct {
	mock.Mock
}

// GetCurrentIngestion provides a mock function with given fields: _a0, _a1
func (_m *MockIngestionServicerServer) GetCurrentIngestion(_a0 context.Context, _a1 *GetCurrentIngestionRequest) (*GetCurrentIngestionResponse, error) {
	ret := _m.Called(_a0, _a1)

	if len(ret) == 0 {
		panic("no return value specified for GetCurrentIngestion")
	}

	var r0 *GetCurrentIngestionResponse
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *GetCurrentIngestionRequest) (*GetCurrentIngestionResponse, error)); ok {
		return rf(_a0, _a1)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *GetCurrentIngestionRequest) *GetCurrentIngestionResponse); ok {
		r0 = rf(_a0, _a1)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*GetCurrentIngestionResponse)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *GetCurrentIngestionRequest) error); ok {
		r1 = rf(_a0, _a1)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// UpdateIngestion provides a mock function with given fields: _a0, _a1
func (_m *MockIngestionServicerServer) UpdateIngestion(_a0 *UpdateIngestionRequest, _a1 grpc.ServerStreamingServer[UpdateIngestionResponse]) error {
	ret := _m.Called(_a0, _a1)

	if len(ret) == 0 {
		panic("no return value specified for UpdateIngestion")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(*UpdateIngestionRequest, grpc.ServerStreamingServer[UpdateIngestionResponse]) error); ok {
		r0 = rf(_a0, _a1)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// mustEmbedUnimplementedIngestionServicerServer provides a mock function with given fields:
func (_m *MockIngestionServicerServer) mustEmbedUnimplementedIngestionServicerServer() {
	_m.Called()
}

// NewMockIngestionServicerServer creates a new instance of MockIngestionServicerServer. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewMockIngestionServicerServer(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockIngestionServicerServer {
	mock := &MockIngestionServicerServer{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
