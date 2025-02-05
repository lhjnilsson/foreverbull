// Code generated by mockery v2.46.3. DO NOT EDIT.

package finance

import (
	context "context"

	grpc "google.golang.org/grpc"

	mock "github.com/stretchr/testify/mock"
)

// MockMarketdataClient is an autogenerated mock type for the MarketdataClient type
type MockMarketdataClient struct {
	mock.Mock
}

// DownloadHistoricalData provides a mock function with given fields: ctx, in, opts
func (_m *MockMarketdataClient) DownloadHistoricalData(ctx context.Context, in *DownloadHistoricalDataRequest, opts ...grpc.CallOption) (*DownloadHistoricalDataResponse, error) {
	_va := make([]interface{}, len(opts))
	for _i := range opts {
		_va[_i] = opts[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, ctx, in)
	_ca = append(_ca, _va...)
	ret := _m.Called(_ca...)

	if len(ret) == 0 {
		panic("no return value specified for DownloadHistoricalData")
	}

	var r0 *DownloadHistoricalDataResponse
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *DownloadHistoricalDataRequest, ...grpc.CallOption) (*DownloadHistoricalDataResponse, error)); ok {
		return rf(ctx, in, opts...)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *DownloadHistoricalDataRequest, ...grpc.CallOption) *DownloadHistoricalDataResponse); ok {
		r0 = rf(ctx, in, opts...)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*DownloadHistoricalDataResponse)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *DownloadHistoricalDataRequest, ...grpc.CallOption) error); ok {
		r1 = rf(ctx, in, opts...)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetAsset provides a mock function with given fields: ctx, in, opts
func (_m *MockMarketdataClient) GetAsset(ctx context.Context, in *GetAssetRequest, opts ...grpc.CallOption) (*GetAssetResponse, error) {
	_va := make([]interface{}, len(opts))
	for _i := range opts {
		_va[_i] = opts[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, ctx, in)
	_ca = append(_ca, _va...)
	ret := _m.Called(_ca...)

	if len(ret) == 0 {
		panic("no return value specified for GetAsset")
	}

	var r0 *GetAssetResponse
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *GetAssetRequest, ...grpc.CallOption) (*GetAssetResponse, error)); ok {
		return rf(ctx, in, opts...)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *GetAssetRequest, ...grpc.CallOption) *GetAssetResponse); ok {
		r0 = rf(ctx, in, opts...)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*GetAssetResponse)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *GetAssetRequest, ...grpc.CallOption) error); ok {
		r1 = rf(ctx, in, opts...)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetIndex provides a mock function with given fields: ctx, in, opts
func (_m *MockMarketdataClient) GetIndex(ctx context.Context, in *GetIndexRequest, opts ...grpc.CallOption) (*GetIndexResponse, error) {
	_va := make([]interface{}, len(opts))
	for _i := range opts {
		_va[_i] = opts[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, ctx, in)
	_ca = append(_ca, _va...)
	ret := _m.Called(_ca...)

	if len(ret) == 0 {
		panic("no return value specified for GetIndex")
	}

	var r0 *GetIndexResponse
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *GetIndexRequest, ...grpc.CallOption) (*GetIndexResponse, error)); ok {
		return rf(ctx, in, opts...)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *GetIndexRequest, ...grpc.CallOption) *GetIndexResponse); ok {
		r0 = rf(ctx, in, opts...)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*GetIndexResponse)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *GetIndexRequest, ...grpc.CallOption) error); ok {
		r1 = rf(ctx, in, opts...)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// NewMockMarketdataClient creates a new instance of MockMarketdataClient. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewMockMarketdataClient(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockMarketdataClient {
	mock := &MockMarketdataClient{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
