// Code generated by mockery v2.46.3. DO NOT EDIT.

package engine

import (
	context "context"

	financepb "github.com/lhjnilsson/foreverbull/pkg/finance/pb"
	mock "github.com/stretchr/testify/mock"

	pb "github.com/lhjnilsson/foreverbull/pkg/backtest/pb"

	storage "github.com/lhjnilsson/foreverbull/internal/storage"

	worker "github.com/lhjnilsson/foreverbull/pkg/service/worker"
)

// MockEngine is an autogenerated mock type for the Engine type
type MockEngine struct {
	mock.Mock
}

// DownloadIngestion provides a mock function with given fields: ctx, object
func (_m *MockEngine) DownloadIngestion(ctx context.Context, object *storage.Object) error {
	ret := _m.Called(ctx, object)

	if len(ret) == 0 {
		panic("no return value specified for DownloadIngestion")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *storage.Object) error); ok {
		r0 = rf(ctx, object)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// GetResult provides a mock function with given fields: ctx
func (_m *MockEngine) GetResult(ctx context.Context) (*pb.GetResultResponse, error) {
	ret := _m.Called(ctx)

	if len(ret) == 0 {
		panic("no return value specified for GetResult")
	}

	var r0 *pb.GetResultResponse
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context) (*pb.GetResultResponse, error)); ok {
		return rf(ctx)
	}
	if rf, ok := ret.Get(0).(func(context.Context) *pb.GetResultResponse); ok {
		r0 = rf(ctx)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*pb.GetResultResponse)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context) error); ok {
		r1 = rf(ctx)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// Ingest provides a mock function with given fields: ctx, ingestion, object
func (_m *MockEngine) Ingest(ctx context.Context, ingestion *pb.Ingestion, object *storage.Object) error {
	ret := _m.Called(ctx, ingestion, object)

	if len(ret) == 0 {
		panic("no return value specified for Ingest")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *pb.Ingestion, *storage.Object) error); ok {
		r0 = rf(ctx, ingestion, object)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// RunBacktest provides a mock function with given fields: ctx, backtest, workers
func (_m *MockEngine) RunBacktest(ctx context.Context, backtest *pb.Backtest, workers worker.Pool) (chan *financepb.Portfolio, error) {
	ret := _m.Called(ctx, backtest, workers)

	if len(ret) == 0 {
		panic("no return value specified for RunBacktest")
	}

	var r0 chan *financepb.Portfolio
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *pb.Backtest, worker.Pool) (chan *financepb.Portfolio, error)); ok {
		return rf(ctx, backtest, workers)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *pb.Backtest, worker.Pool) chan *financepb.Portfolio); ok {
		r0 = rf(ctx, backtest, workers)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(chan *financepb.Portfolio)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *pb.Backtest, worker.Pool) error); ok {
		r1 = rf(ctx, backtest, workers)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// Stop provides a mock function with given fields: ctx
func (_m *MockEngine) Stop(ctx context.Context) error {
	ret := _m.Called(ctx)

	if len(ret) == 0 {
		panic("no return value specified for Stop")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context) error); ok {
		r0 = rf(ctx)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// NewMockEngine creates a new instance of MockEngine. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewMockEngine(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockEngine {
	mock := &MockEngine{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
