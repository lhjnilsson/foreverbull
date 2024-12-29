// Code generated by mockery v2.50.1. DO NOT EDIT.

package finance

import (
	context "context"

	stream "github.com/lhjnilsson/foreverbull/internal/stream"
	mock "github.com/stretchr/testify/mock"
)

// MockStream is an autogenerated mock type for the Stream type
type MockStream struct {
	mock.Mock
}

// CommandSubscriber provides a mock function with given fields: component, method, cb
func (_m *MockStream) CommandSubscriber(component string, method string, cb func(context.Context, stream.Message) error) error {
	ret := _m.Called(component, method, cb)

	if len(ret) == 0 {
		panic("no return value specified for CommandSubscriber")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(string, string, func(context.Context, stream.Message) error) error); ok {
		r0 = rf(component, method, cb)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// Publish provides a mock function with given fields: ctx, message
func (_m *MockStream) Publish(ctx context.Context, message stream.Message) error {
	ret := _m.Called(ctx, message)

	if len(ret) == 0 {
		panic("no return value specified for Publish")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, stream.Message) error); ok {
		r0 = rf(ctx, message)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// RunOrchestration provides a mock function with given fields: ctx, orchestration
func (_m *MockStream) RunOrchestration(ctx context.Context, orchestration *stream.MessageOrchestration) error {
	ret := _m.Called(ctx, orchestration)

	if len(ret) == 0 {
		panic("no return value specified for RunOrchestration")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *stream.MessageOrchestration) error); ok {
		r0 = rf(ctx, orchestration)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// Unsubscribe provides a mock function with no fields
func (_m *MockStream) Unsubscribe() error {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for Unsubscribe")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func() error); ok {
		r0 = rf()
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// NewMockStream creates a new instance of MockStream. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewMockStream(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockStream {
	mock := &MockStream{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
