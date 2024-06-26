// Code generated by mockery v2.18.0. DO NOT EDIT.

package socket

import mock "github.com/stretchr/testify/mock"

// MockReader is an autogenerated mock type for the Reader type
type MockReader struct {
	mock.Mock
}

// Close provides a mock function with given fields:
func (_m *MockReader) Close() error {
	ret := _m.Called()

	var r0 error
	if rf, ok := ret.Get(0).(func() error); ok {
		r0 = rf()
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// Read provides a mock function with given fields:
func (_m *MockReader) Read() ([]byte, error) {
	ret := _m.Called()

	var r0 []byte
	if rf, ok := ret.Get(0).(func() []byte); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]byte)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func() error); ok {
		r1 = rf()
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

type mockConstructorTestingTNewMockReader interface {
	mock.TestingT
	Cleanup(func())
}

// NewMockReader creates a new instance of MockReader. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewMockReader(t mockConstructorTestingTNewMockReader) *MockReader {
	mock := &MockReader{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
