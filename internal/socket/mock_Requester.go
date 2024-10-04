// Code generated by mockery v2.18.0. DO NOT EDIT.

package socket

import (
	mock "github.com/stretchr/testify/mock"
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
)

// MockRequester is an autogenerated mock type for the Requester type
type MockRequester struct {
	mock.Mock
}

// Close provides a mock function with given fields:
func (_m *MockRequester) Close() error {
	ret := _m.Called()

	var r0 error
	if rf, ok := ret.Get(0).(func() error); ok {
		r0 = rf()
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// GetHost provides a mock function with given fields:
func (_m *MockRequester) GetHost() string {
	ret := _m.Called()

	var r0 string
	if rf, ok := ret.Get(0).(func() string); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(string)
	}

	return r0
}

// GetPort provides a mock function with given fields:
func (_m *MockRequester) GetPort() int {
	ret := _m.Called()

	var r0 int
	if rf, ok := ret.Get(0).(func() int); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(int)
	}

	return r0
}

// Request provides a mock function with given fields: msg, reply, opts
func (_m *MockRequester) Request(msg protoreflect.ProtoMessage, reply protoreflect.ProtoMessage, opts ...func(OptionSetter) error) error {
	_va := make([]interface{}, len(opts))
	for _i := range opts {
		_va[_i] = opts[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, msg, reply)
	_ca = append(_ca, _va...)
	ret := _m.Called(_ca...)

	var r0 error
	if rf, ok := ret.Get(0).(func(protoreflect.ProtoMessage, protoreflect.ProtoMessage, ...func(OptionSetter) error) error); ok {
		r0 = rf(msg, reply, opts...)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

type mockConstructorTestingTNewMockRequester interface {
	mock.TestingT
	Cleanup(func())
}

// NewMockRequester creates a new instance of MockRequester. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewMockRequester(t mockConstructorTestingTNewMockRequester) *MockRequester {
	mock := &MockRequester{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
