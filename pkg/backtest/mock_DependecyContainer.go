// Code generated by mockery v2.50.1. DO NOT EDIT.

package backtest

import (
	context "context"

	stream "github.com/lhjnilsson/foreverbull/internal/stream"
	mock "github.com/stretchr/testify/mock"
)

// MockDependecyContainer is an autogenerated mock type for the DependecyContainer type
type MockDependecyContainer struct {
	mock.Mock
}

// AddMethod provides a mock function with given fields: key, f
func (_m *MockDependecyContainer) AddMethod(key stream.Dependency, f func(context.Context, stream.Message) (interface{}, error)) {
	_m.Called(key, f)
}

// AddSingleton provides a mock function with given fields: key, v
func (_m *MockDependecyContainer) AddSingleton(key stream.Dependency, v interface{}) {
	_m.Called(key, v)
}

// NewMockDependecyContainer creates a new instance of MockDependecyContainer. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewMockDependecyContainer(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockDependecyContainer {
	mock := &MockDependecyContainer{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
