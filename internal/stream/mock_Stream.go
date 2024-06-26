// Code generated by mockery v2.18.0. DO NOT EDIT.

package stream

import (
	context "context"

	mock "github.com/stretchr/testify/mock"
)

// MockStream is an autogenerated mock type for the Stream type
type MockStream struct {
	mock.Mock
}

// CommandSubscriber provides a mock function with given fields: component, method, cb
func (_m *MockStream) CommandSubscriber(component string, method string, cb func(context.Context, Message) error) error {
	ret := _m.Called(component, method, cb)

	var r0 error
	if rf, ok := ret.Get(0).(func(string, string, func(context.Context, Message) error) error); ok {
		r0 = rf(component, method, cb)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// Publish provides a mock function with given fields: ctx, message
func (_m *MockStream) Publish(ctx context.Context, message Message) error {
	ret := _m.Called(ctx, message)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, Message) error); ok {
		r0 = rf(ctx, message)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// RunOrchestration provides a mock function with given fields: ctx, orchestration
func (_m *MockStream) RunOrchestration(ctx context.Context, orchestration *MessageOrchestration) error {
	ret := _m.Called(ctx, orchestration)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *MessageOrchestration) error); ok {
		r0 = rf(ctx, orchestration)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// Unsubscribe provides a mock function with given fields:
func (_m *MockStream) Unsubscribe() error {
	ret := _m.Called()

	var r0 error
	if rf, ok := ret.Get(0).(func() error); ok {
		r0 = rf()
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

type mockConstructorTestingTNewMockStream interface {
	mock.TestingT
	Cleanup(func())
}

// NewMockStream creates a new instance of MockStream. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewMockStream(t mockConstructorTestingTNewMockStream) *MockStream {
	mock := &MockStream{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
