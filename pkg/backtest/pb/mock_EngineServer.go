// Code generated by mockery v2.18.0. DO NOT EDIT.

package pb

import (
	context "context"

	mock "github.com/stretchr/testify/mock"
)

// MockEngineServer is an autogenerated mock type for the EngineServer type
type MockEngineServer struct {
	mock.Mock
}

// DownloadIngestion provides a mock function with given fields: _a0, _a1
func (_m *MockEngineServer) DownloadIngestion(_a0 context.Context, _a1 *DownloadIngestionRequest) (*DownloadIngestionResponse, error) {
	ret := _m.Called(_a0, _a1)

	var r0 *DownloadIngestionResponse
	if rf, ok := ret.Get(0).(func(context.Context, *DownloadIngestionRequest) *DownloadIngestionResponse); ok {
		r0 = rf(_a0, _a1)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*DownloadIngestionResponse)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, *DownloadIngestionRequest) error); ok {
		r1 = rf(_a0, _a1)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetCurrentPeriod provides a mock function with given fields: _a0, _a1
func (_m *MockEngineServer) GetCurrentPeriod(_a0 context.Context, _a1 *GetCurrentPeriodRequest) (*GetCurrentPeriodResponse, error) {
	ret := _m.Called(_a0, _a1)

	var r0 *GetCurrentPeriodResponse
	if rf, ok := ret.Get(0).(func(context.Context, *GetCurrentPeriodRequest) *GetCurrentPeriodResponse); ok {
		r0 = rf(_a0, _a1)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*GetCurrentPeriodResponse)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, *GetCurrentPeriodRequest) error); ok {
		r1 = rf(_a0, _a1)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetResult provides a mock function with given fields: _a0, _a1
func (_m *MockEngineServer) GetResult(_a0 context.Context, _a1 *GetResultRequest) (*GetResultResponse, error) {
	ret := _m.Called(_a0, _a1)

	var r0 *GetResultResponse
	if rf, ok := ret.Get(0).(func(context.Context, *GetResultRequest) *GetResultResponse); ok {
		r0 = rf(_a0, _a1)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*GetResultResponse)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, *GetResultRequest) error); ok {
		r1 = rf(_a0, _a1)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// Ingest provides a mock function with given fields: _a0, _a1
func (_m *MockEngineServer) Ingest(_a0 context.Context, _a1 *IngestRequest) (*IngestResponse, error) {
	ret := _m.Called(_a0, _a1)

	var r0 *IngestResponse
	if rf, ok := ret.Get(0).(func(context.Context, *IngestRequest) *IngestResponse); ok {
		r0 = rf(_a0, _a1)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*IngestResponse)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, *IngestRequest) error); ok {
		r1 = rf(_a0, _a1)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// PlaceOrdersAndContinue provides a mock function with given fields: _a0, _a1
func (_m *MockEngineServer) PlaceOrdersAndContinue(_a0 context.Context, _a1 *PlaceOrdersAndContinueRequest) (*PlaceOrdersAndContinueResponse, error) {
	ret := _m.Called(_a0, _a1)

	var r0 *PlaceOrdersAndContinueResponse
	if rf, ok := ret.Get(0).(func(context.Context, *PlaceOrdersAndContinueRequest) *PlaceOrdersAndContinueResponse); ok {
		r0 = rf(_a0, _a1)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*PlaceOrdersAndContinueResponse)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, *PlaceOrdersAndContinueRequest) error); ok {
		r1 = rf(_a0, _a1)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// RunBacktest provides a mock function with given fields: _a0, _a1
func (_m *MockEngineServer) RunBacktest(_a0 context.Context, _a1 *RunRequest) (*RunResponse, error) {
	ret := _m.Called(_a0, _a1)

	var r0 *RunResponse
	if rf, ok := ret.Get(0).(func(context.Context, *RunRequest) *RunResponse); ok {
		r0 = rf(_a0, _a1)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*RunResponse)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, *RunRequest) error); ok {
		r1 = rf(_a0, _a1)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// Stop provides a mock function with given fields: _a0, _a1
func (_m *MockEngineServer) Stop(_a0 context.Context, _a1 *StopRequest) (*StopResponse, error) {
	ret := _m.Called(_a0, _a1)

	var r0 *StopResponse
	if rf, ok := ret.Get(0).(func(context.Context, *StopRequest) *StopResponse); ok {
		r0 = rf(_a0, _a1)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*StopResponse)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, *StopRequest) error); ok {
		r1 = rf(_a0, _a1)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// mustEmbedUnimplementedEngineServer provides a mock function with given fields:
func (_m *MockEngineServer) mustEmbedUnimplementedEngineServer() {
	_m.Called()
}

type mockConstructorTestingTNewMockEngineServer interface {
	mock.TestingT
	Cleanup(func())
}

// NewMockEngineServer creates a new instance of MockEngineServer. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewMockEngineServer(t mockConstructorTestingTNewMockEngineServer) *MockEngineServer {
	mock := &MockEngineServer{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
