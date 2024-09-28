// Code generated by mockery v2.18.0. DO NOT EDIT.

package container

import (
	context "context"

	mock "github.com/stretchr/testify/mock"
)

// MockEngine is an autogenerated mock type for the Engine type
type MockEngine struct {
	mock.Mock
}

// PullImage provides a mock function with given fields:
func (_m *MockEngine) PullImage() error {
	ret := _m.Called()

	var r0 error
	if rf, ok := ret.Get(0).(func() error); ok {
		r0 = rf()
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// Start provides a mock function with given fields: ctx, image, name
func (_m *MockEngine) Start(ctx context.Context, image string, name string) (Container, error) {
	ret := _m.Called(ctx, image, name)

	var r0 Container
	if rf, ok := ret.Get(0).(func(context.Context, string, string) Container); ok {
		r0 = rf(ctx, image, name)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(Container)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, string, string) error); ok {
		r1 = rf(ctx, image, name)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// StopAll provides a mock function with given fields: ctx, remove
func (_m *MockEngine) StopAll(ctx context.Context, remove bool) error {
	ret := _m.Called(ctx, remove)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, bool) error); ok {
		r0 = rf(ctx, remove)
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
