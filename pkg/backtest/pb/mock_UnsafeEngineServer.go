// Code generated by mockery v2.46.3. DO NOT EDIT.

package pb

import mock "github.com/stretchr/testify/mock"

// MockUnsafeEngineServer is an autogenerated mock type for the UnsafeEngineServer type
type MockUnsafeEngineServer struct {
	mock.Mock
}

// mustEmbedUnimplementedEngineServer provides a mock function with given fields:
func (_m *MockUnsafeEngineServer) mustEmbedUnimplementedEngineServer() {
	_m.Called()
}

// NewMockUnsafeEngineServer creates a new instance of MockUnsafeEngineServer. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewMockUnsafeEngineServer(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockUnsafeEngineServer {
	mock := &MockUnsafeEngineServer{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
