// Code generated by mockery v2.46.3. DO NOT EDIT.

package strategy

import mock "github.com/stretchr/testify/mock"

// MockUnsafeStrategyServicerServer is an autogenerated mock type for the UnsafeStrategyServicerServer type
type MockUnsafeStrategyServicerServer struct {
	mock.Mock
}

// mustEmbedUnimplementedStrategyServicerServer provides a mock function with given fields:
func (_m *MockUnsafeStrategyServicerServer) mustEmbedUnimplementedStrategyServicerServer() {
	_m.Called()
}

// NewMockUnsafeStrategyServicerServer creates a new instance of MockUnsafeStrategyServicerServer. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewMockUnsafeStrategyServicerServer(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockUnsafeStrategyServicerServer {
	mock := &MockUnsafeStrategyServicerServer{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
