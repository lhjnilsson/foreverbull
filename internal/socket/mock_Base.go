// Code generated by mockery v2.46.3. DO NOT EDIT.

package socket

import mock "github.com/stretchr/testify/mock"

// MockBase is an autogenerated mock type for the Base type
type MockBase struct {
	mock.Mock
}

// Close provides a mock function with given fields:
func (_m *MockBase) Close() error {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for Close")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func() error); ok {
		r0 = rf()
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// GetHost provides a mock function with given fields:
func (_m *MockBase) GetHost() string {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for GetHost")
	}

	var r0 string
	if rf, ok := ret.Get(0).(func() string); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(string)
	}

	return r0
}

// GetPort provides a mock function with given fields:
func (_m *MockBase) GetPort() int {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for GetPort")
	}

	var r0 int
	if rf, ok := ret.Get(0).(func() int); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(int)
	}

	return r0
}

// NewMockBase creates a new instance of MockBase. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewMockBase(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockBase {
	mock := &MockBase{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
