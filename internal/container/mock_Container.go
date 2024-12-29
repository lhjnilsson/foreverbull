// Code generated by mockery v2.50.1. DO NOT EDIT.

package container

import mock "github.com/stretchr/testify/mock"

// MockContainer is an autogenerated mock type for the Container type
type MockContainer struct {
	mock.Mock
}

// GetConnectionString provides a mock function with no fields
func (_m *MockContainer) GetConnectionString() (string, error) {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for GetConnectionString")
	}

	var r0 string
	var r1 error
	if rf, ok := ret.Get(0).(func() (string, error)); ok {
		return rf()
	}
	if rf, ok := ret.Get(0).(func() string); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(string)
	}

	if rf, ok := ret.Get(1).(func() error); ok {
		r1 = rf()
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetHealth provides a mock function with no fields
func (_m *MockContainer) GetHealth() (string, error) {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for GetHealth")
	}

	var r0 string
	var r1 error
	if rf, ok := ret.Get(0).(func() (string, error)); ok {
		return rf()
	}
	if rf, ok := ret.Get(0).(func() string); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(string)
	}

	if rf, ok := ret.Get(1).(func() error); ok {
		r1 = rf()
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetIpAddress provides a mock function with no fields
func (_m *MockContainer) GetIpAddress() (string, error) {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for GetIpAddress")
	}

	var r0 string
	var r1 error
	if rf, ok := ret.Get(0).(func() (string, error)); ok {
		return rf()
	}
	if rf, ok := ret.Get(0).(func() string); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(string)
	}

	if rf, ok := ret.Get(1).(func() error); ok {
		r1 = rf()
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetStatus provides a mock function with no fields
func (_m *MockContainer) GetStatus() (string, error) {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for GetStatus")
	}

	var r0 string
	var r1 error
	if rf, ok := ret.Get(0).(func() (string, error)); ok {
		return rf()
	}
	if rf, ok := ret.Get(0).(func() string); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(string)
	}

	if rf, ok := ret.Get(1).(func() error); ok {
		r1 = rf()
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// Stop provides a mock function with no fields
func (_m *MockContainer) Stop() error {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for Stop")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func() error); ok {
		r0 = rf()
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// NewMockContainer creates a new instance of MockContainer. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewMockContainer(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockContainer {
	mock := &MockContainer{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
