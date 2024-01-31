// Code generated by mockery v2.18.0. DO NOT EDIT.

package mocks

import (
	context "context"

	stream "github.com/lhjnilsson/foreverbull/internal/stream"
	mock "github.com/stretchr/testify/mock"
)

// Stream is an autogenerated mock type for the Stream type
type Stream struct {
	mock.Mock
}

// CommandSubscriber provides a mock function with given fields: component, method, cb
func (_m *Stream) CommandSubscriber(component string, method string, cb func(context.Context, stream.Message) error) error {
	ret := _m.Called(component, method, cb)

	var r0 error
	if rf, ok := ret.Get(0).(func(string, string, func(context.Context, stream.Message) error) error); ok {
		r0 = rf(component, method, cb)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// CreateMessage provides a mock function with given fields: ctx, message
func (_m *Stream) CreateMessage(ctx context.Context, message stream.Message) (stream.Message, error) {
	ret := _m.Called(ctx, message)

	var r0 stream.Message
	if rf, ok := ret.Get(0).(func(context.Context, stream.Message) stream.Message); ok {
		r0 = rf(ctx, message)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(stream.Message)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, stream.Message) error); ok {
		r1 = rf(ctx, message)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// CreateOrchestration provides a mock function with given fields: ctx, orchestration
func (_m *Stream) CreateOrchestration(ctx context.Context, orchestration *stream.MessageOrchestration) error {
	ret := _m.Called(ctx, orchestration)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *stream.MessageOrchestration) error); ok {
		r0 = rf(ctx, orchestration)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// Publish provides a mock function with given fields: ctx, message
func (_m *Stream) Publish(ctx context.Context, message stream.Message) error {
	ret := _m.Called(ctx, message)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, stream.Message) error); ok {
		r0 = rf(ctx, message)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// RunOrchestration provides a mock function with given fields: ctx, orchestrationID
func (_m *Stream) RunOrchestration(ctx context.Context, orchestrationID string) error {
	ret := _m.Called(ctx, orchestrationID)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string) error); ok {
		r0 = rf(ctx, orchestrationID)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// Unsubscribe provides a mock function with given fields:
func (_m *Stream) Unsubscribe() error {
	ret := _m.Called()

	var r0 error
	if rf, ok := ret.Get(0).(func() error); ok {
		r0 = rf()
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

type mockConstructorTestingTNewStream interface {
	mock.TestingT
	Cleanup(func())
}

// NewStream creates a new instance of Stream. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewStream(t mockConstructorTestingTNewStream) *Stream {
	mock := &Stream{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
