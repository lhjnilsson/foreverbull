// Code generated by mockery v2.18.0. DO NOT EDIT.

package worker

import mock "github.com/stretchr/testify/mock"

// mockContainerDataTypes is an autogenerated mock type for the containerDataTypes type
type mockContainerDataTypes struct {
	mock.Mock
}

type mockConstructorTestingTnewMockContainerDataTypes interface {
	mock.TestingT
	Cleanup(func())
}

// newMockContainerDataTypes creates a new instance of mockContainerDataTypes. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func newMockContainerDataTypes(t mockConstructorTestingTnewMockContainerDataTypes) *mockContainerDataTypes {
	mock := &mockContainerDataTypes{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
