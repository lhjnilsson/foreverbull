// Code generated by mockery v2.18.0. DO NOT EDIT.

package pb

import mock "github.com/stretchr/testify/mock"

// MockUnsafeWorkerServer is an autogenerated mock type for the UnsafeWorkerServer type
type MockUnsafeWorkerServer struct {
	mock.Mock
}

// mustEmbedUnimplementedWorkerServer provides a mock function with given fields:
func (_m *MockUnsafeWorkerServer) mustEmbedUnimplementedWorkerServer() {
	_m.Called()
}

type mockConstructorTestingTNewMockUnsafeWorkerServer interface {
	mock.TestingT
	Cleanup(func())
}

// NewMockUnsafeWorkerServer creates a new instance of MockUnsafeWorkerServer. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewMockUnsafeWorkerServer(t mockConstructorTestingTNewMockUnsafeWorkerServer) *MockUnsafeWorkerServer {
	mock := &MockUnsafeWorkerServer{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
