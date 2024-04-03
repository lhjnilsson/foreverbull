// Code generated by mockery v2.33.2. DO NOT EDIT.

package mocks

import (
	gin "github.com/gin-gonic/gin"

	mock "github.com/stretchr/testify/mock"

	pgxpool "github.com/jackc/pgx/v5/pgxpool"
)

// Route is an autogenerated mock type for the Route type
type Route struct {
	mock.Mock
}

// Path provides a mock function with given fields:
func (_m *Route) Path() string {
	ret := _m.Called()

	var r0 string
	if rf, ok := ret.Get(0).(func() string); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(string)
	}

	return r0
}

// Setup provides a mock function with given fields: group, pool
func (_m *Route) Setup(group *gin.RouterGroup, pool *pgxpool.Pool) error {
	ret := _m.Called(group, pool)

	var r0 error
	if rf, ok := ret.Get(0).(func(*gin.RouterGroup, *pgxpool.Pool) error); ok {
		r0 = rf(group, pool)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// NewRoute creates a new instance of Route. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewRoute(t interface {
	mock.TestingT
	Cleanup(func())
}) *Route {
	mock := &Route{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
