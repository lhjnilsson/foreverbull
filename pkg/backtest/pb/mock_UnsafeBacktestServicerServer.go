// Code generated by mockery v2.50.1. DO NOT EDIT.

package pb

import mock "github.com/stretchr/testify/mock"

// MockUnsafeBacktestServicerServer is an autogenerated mock type for the UnsafeBacktestServicerServer type
type MockUnsafeBacktestServicerServer struct {
	mock.Mock
}

// mustEmbedUnimplementedBacktestServicerServer provides a mock function with no fields
func (_m *MockUnsafeBacktestServicerServer) mustEmbedUnimplementedBacktestServicerServer() {
	_m.Called()
}

// NewMockUnsafeBacktestServicerServer creates a new instance of MockUnsafeBacktestServicerServer. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewMockUnsafeBacktestServicerServer(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockUnsafeBacktestServicerServer {
	mock := &MockUnsafeBacktestServicerServer{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
