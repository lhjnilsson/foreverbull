// Code generated by mockery v2.50.1. DO NOT EDIT.

package pb

import mock "github.com/stretchr/testify/mock"

// MockUnsafeMarketdataServer is an autogenerated mock type for the UnsafeMarketdataServer type
type MockUnsafeMarketdataServer struct {
	mock.Mock
}

// mustEmbedUnimplementedMarketdataServer provides a mock function with no fields
func (_m *MockUnsafeMarketdataServer) mustEmbedUnimplementedMarketdataServer() {
	_m.Called()
}

// NewMockUnsafeMarketdataServer creates a new instance of MockUnsafeMarketdataServer. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewMockUnsafeMarketdataServer(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockUnsafeMarketdataServer {
	mock := &MockUnsafeMarketdataServer{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
