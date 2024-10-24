// Code generated by mockery v2.18.0. DO NOT EDIT.

package engine

import (
	context "context"

	storage "github.com/lhjnilsson/foreverbull/internal/storage"
	pb "github.com/lhjnilsson/foreverbull/pkg/backtest/pb"
	financepb "github.com/lhjnilsson/foreverbull/pkg/finance/pb"
	worker "github.com/lhjnilsson/foreverbull/pkg/service/worker"
	mock "github.com/stretchr/testify/mock"
)

// MockEngine is an autogenerated mock type for the Engine type
type MockEngine struct {
	mock.Mock
}

// DownloadIngestion provides a mock function with given fields: _a0, _a1
func (_m *MockEngine) DownloadIngestion(_a0 context.Context, _a1 *storage.Object) error {
	ret := _m.Called(_a0, _a1)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *storage.Object) error); ok {
		r0 = rf(_a0, _a1)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// GetResult provides a mock function with given fields: ctx
func (_m *MockEngine) GetResult(ctx context.Context) (*pb.GetResultResponse, error) {
	ret := _m.Called(ctx)

	var r0 *pb.GetResultResponse
	if rf, ok := ret.Get(0).(func(context.Context) *pb.GetResultResponse); ok {
		r0 = rf(ctx)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*pb.GetResultResponse)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context) error); ok {
		r1 = rf(ctx)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// Ingest provides a mock function with given fields: _a0, _a1, _a2
func (_m *MockEngine) Ingest(_a0 context.Context, _a1 *pb.Ingestion, _a2 *storage.Object) error {
	ret := _m.Called(_a0, _a1, _a2)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *pb.Ingestion, *storage.Object) error); ok {
		r0 = rf(_a0, _a1, _a2)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// RunBacktest provides a mock function with given fields: _a0, _a1, _a2
func (_m *MockEngine) RunBacktest(_a0 context.Context, _a1 *pb.Backtest, _a2 worker.Pool) (chan *financepb.Portfolio, error) {
	ret := _m.Called(_a0, _a1, _a2)

	var r0 chan *financepb.Portfolio
	if rf, ok := ret.Get(0).(func(context.Context, *pb.Backtest, worker.Pool) chan *financepb.Portfolio); ok {
		r0 = rf(_a0, _a1, _a2)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(chan *financepb.Portfolio)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, *pb.Backtest, worker.Pool) error); ok {
		r1 = rf(_a0, _a1, _a2)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// Stop provides a mock function with given fields: _a0
func (_m *MockEngine) Stop(_a0 context.Context) error {
	ret := _m.Called(_a0)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context) error); ok {
		r0 = rf(_a0)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

type mockConstructorTestingTNewMockEngine interface {
	mock.TestingT
	Cleanup(func())
}

// NewMockEngine creates a new instance of MockEngine. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewMockEngine(t mockConstructorTestingTNewMockEngine) *MockEngine {
	mock := &MockEngine{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
