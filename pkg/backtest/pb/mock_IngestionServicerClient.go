// Code generated by mockery v2.18.0. DO NOT EDIT.

package pb

import (
	context "context"

	grpc "google.golang.org/grpc"

	mock "github.com/stretchr/testify/mock"
)

// MockIngestionServicerClient is an autogenerated mock type for the IngestionServicerClient type
type MockIngestionServicerClient struct {
	mock.Mock
}

// CreateIngestion provides a mock function with given fields: ctx, in, opts
func (_m *MockIngestionServicerClient) CreateIngestion(ctx context.Context, in *CreateIngestionRequest, opts ...grpc.CallOption) (*CreateIngestionResponse, error) {
	_va := make([]interface{}, len(opts))
	for _i := range opts {
		_va[_i] = opts[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, ctx, in)
	_ca = append(_ca, _va...)
	ret := _m.Called(_ca...)

	var r0 *CreateIngestionResponse
	if rf, ok := ret.Get(0).(func(context.Context, *CreateIngestionRequest, ...grpc.CallOption) *CreateIngestionResponse); ok {
		r0 = rf(ctx, in, opts...)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*CreateIngestionResponse)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, *CreateIngestionRequest, ...grpc.CallOption) error); ok {
		r1 = rf(ctx, in, opts...)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetCurrentIngestion provides a mock function with given fields: ctx, in, opts
func (_m *MockIngestionServicerClient) GetCurrentIngestion(ctx context.Context, in *GetCurrentIngestionRequest, opts ...grpc.CallOption) (*GetCurrentIngestionResponse, error) {
	_va := make([]interface{}, len(opts))
	for _i := range opts {
		_va[_i] = opts[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, ctx, in)
	_ca = append(_ca, _va...)
	ret := _m.Called(_ca...)

	var r0 *GetCurrentIngestionResponse
	if rf, ok := ret.Get(0).(func(context.Context, *GetCurrentIngestionRequest, ...grpc.CallOption) *GetCurrentIngestionResponse); ok {
		r0 = rf(ctx, in, opts...)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*GetCurrentIngestionResponse)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, *GetCurrentIngestionRequest, ...grpc.CallOption) error); ok {
		r1 = rf(ctx, in, opts...)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

type mockConstructorTestingTNewMockIngestionServicerClient interface {
	mock.TestingT
	Cleanup(func())
}

// NewMockIngestionServicerClient creates a new instance of MockIngestionServicerClient. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewMockIngestionServicerClient(t mockConstructorTestingTNewMockIngestionServicerClient) *MockIngestionServicerClient {
	mock := &MockIngestionServicerClient{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
