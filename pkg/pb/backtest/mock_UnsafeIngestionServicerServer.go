// Code generated by mockery v2.46.3. DO NOT EDIT.

package backtest

import mock "github.com/stretchr/testify/mock"

// MockUnsafeIngestionServicerServer is an autogenerated mock type for the UnsafeIngestionServicerServer type
type MockUnsafeIngestionServicerServer struct {
	mock.Mock
}

// mustEmbedUnimplementedIngestionServicerServer provides a mock function with given fields:
func (_m *MockUnsafeIngestionServicerServer) mustEmbedUnimplementedIngestionServicerServer() {
	_m.Called()
}

// NewMockUnsafeIngestionServicerServer creates a new instance of MockUnsafeIngestionServicerServer. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewMockUnsafeIngestionServicerServer(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockUnsafeIngestionServicerServer {
	mock := &MockUnsafeIngestionServicerServer{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
