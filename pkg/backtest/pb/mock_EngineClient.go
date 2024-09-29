// Code generated by mockery v2.18.0. DO NOT EDIT.

package pb

import (
	context "context"

	grpc "google.golang.org/grpc"

	mock "github.com/stretchr/testify/mock"
)

// MockEngineClient is an autogenerated mock type for the EngineClient type
type MockEngineClient struct {
	mock.Mock
}

// DownloadIngestion provides a mock function with given fields: ctx, in, opts
func (_m *MockEngineClient) DownloadIngestion(ctx context.Context, in *DownloadIngestionRequest, opts ...grpc.CallOption) (*DownloadIngestionResponse, error) {
	_va := make([]interface{}, len(opts))
	for _i := range opts {
		_va[_i] = opts[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, ctx, in)
	_ca = append(_ca, _va...)
	ret := _m.Called(_ca...)

	var r0 *DownloadIngestionResponse
	if rf, ok := ret.Get(0).(func(context.Context, *DownloadIngestionRequest, ...grpc.CallOption) *DownloadIngestionResponse); ok {
		r0 = rf(ctx, in, opts...)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*DownloadIngestionResponse)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, *DownloadIngestionRequest, ...grpc.CallOption) error); ok {
		r1 = rf(ctx, in, opts...)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetCurrentPeriod provides a mock function with given fields: ctx, in, opts
func (_m *MockEngineClient) GetCurrentPeriod(ctx context.Context, in *GetCurrentPeriodRequest, opts ...grpc.CallOption) (*GetCurrentPeriodResponse, error) {
	_va := make([]interface{}, len(opts))
	for _i := range opts {
		_va[_i] = opts[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, ctx, in)
	_ca = append(_ca, _va...)
	ret := _m.Called(_ca...)

	var r0 *GetCurrentPeriodResponse
	if rf, ok := ret.Get(0).(func(context.Context, *GetCurrentPeriodRequest, ...grpc.CallOption) *GetCurrentPeriodResponse); ok {
		r0 = rf(ctx, in, opts...)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*GetCurrentPeriodResponse)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, *GetCurrentPeriodRequest, ...grpc.CallOption) error); ok {
		r1 = rf(ctx, in, opts...)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetResult provides a mock function with given fields: ctx, in, opts
func (_m *MockEngineClient) GetResult(ctx context.Context, in *GetResultRequest, opts ...grpc.CallOption) (*GetResultResponse, error) {
	_va := make([]interface{}, len(opts))
	for _i := range opts {
		_va[_i] = opts[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, ctx, in)
	_ca = append(_ca, _va...)
	ret := _m.Called(_ca...)

	var r0 *GetResultResponse
	if rf, ok := ret.Get(0).(func(context.Context, *GetResultRequest, ...grpc.CallOption) *GetResultResponse); ok {
		r0 = rf(ctx, in, opts...)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*GetResultResponse)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, *GetResultRequest, ...grpc.CallOption) error); ok {
		r1 = rf(ctx, in, opts...)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// Ingest provides a mock function with given fields: ctx, in, opts
func (_m *MockEngineClient) Ingest(ctx context.Context, in *IngestRequest, opts ...grpc.CallOption) (*IngestResponse, error) {
	_va := make([]interface{}, len(opts))
	for _i := range opts {
		_va[_i] = opts[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, ctx, in)
	_ca = append(_ca, _va...)
	ret := _m.Called(_ca...)

	var r0 *IngestResponse
	if rf, ok := ret.Get(0).(func(context.Context, *IngestRequest, ...grpc.CallOption) *IngestResponse); ok {
		r0 = rf(ctx, in, opts...)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*IngestResponse)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, *IngestRequest, ...grpc.CallOption) error); ok {
		r1 = rf(ctx, in, opts...)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// PlaceOrdersAndContinue provides a mock function with given fields: ctx, in, opts
func (_m *MockEngineClient) PlaceOrdersAndContinue(ctx context.Context, in *PlaceOrdersAndContinueRequest, opts ...grpc.CallOption) (*PlaceOrdersAndContinueResponse, error) {
	_va := make([]interface{}, len(opts))
	for _i := range opts {
		_va[_i] = opts[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, ctx, in)
	_ca = append(_ca, _va...)
	ret := _m.Called(_ca...)

	var r0 *PlaceOrdersAndContinueResponse
	if rf, ok := ret.Get(0).(func(context.Context, *PlaceOrdersAndContinueRequest, ...grpc.CallOption) *PlaceOrdersAndContinueResponse); ok {
		r0 = rf(ctx, in, opts...)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*PlaceOrdersAndContinueResponse)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, *PlaceOrdersAndContinueRequest, ...grpc.CallOption) error); ok {
		r1 = rf(ctx, in, opts...)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// RunBacktest provides a mock function with given fields: ctx, in, opts
func (_m *MockEngineClient) RunBacktest(ctx context.Context, in *RunRequest, opts ...grpc.CallOption) (*RunResponse, error) {
	_va := make([]interface{}, len(opts))
	for _i := range opts {
		_va[_i] = opts[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, ctx, in)
	_ca = append(_ca, _va...)
	ret := _m.Called(_ca...)

	var r0 *RunResponse
	if rf, ok := ret.Get(0).(func(context.Context, *RunRequest, ...grpc.CallOption) *RunResponse); ok {
		r0 = rf(ctx, in, opts...)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*RunResponse)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, *RunRequest, ...grpc.CallOption) error); ok {
		r1 = rf(ctx, in, opts...)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// Stop provides a mock function with given fields: ctx, in, opts
func (_m *MockEngineClient) Stop(ctx context.Context, in *StopRequest, opts ...grpc.CallOption) (*StopResponse, error) {
	_va := make([]interface{}, len(opts))
	for _i := range opts {
		_va[_i] = opts[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, ctx, in)
	_ca = append(_ca, _va...)
	ret := _m.Called(_ca...)

	var r0 *StopResponse
	if rf, ok := ret.Get(0).(func(context.Context, *StopRequest, ...grpc.CallOption) *StopResponse); ok {
		r0 = rf(ctx, in, opts...)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*StopResponse)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, *StopRequest, ...grpc.CallOption) error); ok {
		r1 = rf(ctx, in, opts...)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

type mockConstructorTestingTNewMockEngineClient interface {
	mock.TestingT
	Cleanup(func())
}

// NewMockEngineClient creates a new instance of MockEngineClient. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewMockEngineClient(t mockConstructorTestingTNewMockEngineClient) *MockEngineClient {
	mock := &MockEngineClient{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}