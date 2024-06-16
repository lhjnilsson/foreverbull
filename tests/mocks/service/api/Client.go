// Code generated by mockery v2.18.0. DO NOT EDIT.

package mocks

import (
	context "context"

	api "github.com/lhjnilsson/foreverbull/pkg/service/api"

	mock "github.com/stretchr/testify/mock"
)

// Client is an autogenerated mock type for the Client type
type Client struct {
	mock.Mock
}

// ConfigureInstance provides a mock function with given fields: ctx, InstanceID, config
func (_m *Client) ConfigureInstance(ctx context.Context, InstanceID string, config *api.ConfigureInstanceRequest) (*api.InstanceResponse, error) {
	ret := _m.Called(ctx, InstanceID, config)

	var r0 *api.InstanceResponse
	if rf, ok := ret.Get(0).(func(context.Context, string, *api.ConfigureInstanceRequest) *api.InstanceResponse); ok {
		r0 = rf(ctx, InstanceID, config)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*api.InstanceResponse)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, string, *api.ConfigureInstanceRequest) error); ok {
		r1 = rf(ctx, InstanceID, config)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// CreateService provides a mock function with given fields: ctx, service
func (_m *Client) CreateService(ctx context.Context, service *api.CreateServiceRequest) (*api.ServiceResponse, error) {
	ret := _m.Called(ctx, service)

	var r0 *api.ServiceResponse
	if rf, ok := ret.Get(0).(func(context.Context, *api.CreateServiceRequest) *api.ServiceResponse); ok {
		r0 = rf(ctx, service)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*api.ServiceResponse)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, *api.CreateServiceRequest) error); ok {
		r1 = rf(ctx, service)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// DownloadImage provides a mock function with given fields: ctx, image
func (_m *Client) DownloadImage(ctx context.Context, image string) (*api.ImageResponse, error) {
	ret := _m.Called(ctx, image)

	var r0 *api.ImageResponse
	if rf, ok := ret.Get(0).(func(context.Context, string) *api.ImageResponse); ok {
		r0 = rf(ctx, image)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*api.ImageResponse)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, string) error); ok {
		r1 = rf(ctx, image)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetImage provides a mock function with given fields: ctx, image
func (_m *Client) GetImage(ctx context.Context, image string) (*api.ImageResponse, error) {
	ret := _m.Called(ctx, image)

	var r0 *api.ImageResponse
	if rf, ok := ret.Get(0).(func(context.Context, string) *api.ImageResponse); ok {
		r0 = rf(ctx, image)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*api.ImageResponse)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, string) error); ok {
		r1 = rf(ctx, image)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetInstance provides a mock function with given fields: ctx, InstanceID
func (_m *Client) GetInstance(ctx context.Context, InstanceID string) (*api.InstanceResponse, error) {
	ret := _m.Called(ctx, InstanceID)

	var r0 *api.InstanceResponse
	if rf, ok := ret.Get(0).(func(context.Context, string) *api.InstanceResponse); ok {
		r0 = rf(ctx, InstanceID)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*api.InstanceResponse)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, string) error); ok {
		r1 = rf(ctx, InstanceID)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetService provides a mock function with given fields: ctx, image
func (_m *Client) GetService(ctx context.Context, image string) (*api.ServiceResponse, error) {
	ret := _m.Called(ctx, image)

	var r0 *api.ServiceResponse
	if rf, ok := ret.Get(0).(func(context.Context, string) *api.ServiceResponse); ok {
		r0 = rf(ctx, image)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*api.ServiceResponse)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, string) error); ok {
		r1 = rf(ctx, image)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ListInstances provides a mock function with given fields: ctx, image
func (_m *Client) ListInstances(ctx context.Context, image string) (*[]api.InstanceResponse, error) {
	ret := _m.Called(ctx, image)

	var r0 *[]api.InstanceResponse
	if rf, ok := ret.Get(0).(func(context.Context, string) *[]api.InstanceResponse); ok {
		r0 = rf(ctx, image)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*[]api.InstanceResponse)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, string) error); ok {
		r1 = rf(ctx, image)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ListServices provides a mock function with given fields: ctx
func (_m *Client) ListServices(ctx context.Context) (*[]api.ServiceResponse, error) {
	ret := _m.Called(ctx)

	var r0 *[]api.ServiceResponse
	if rf, ok := ret.Get(0).(func(context.Context) *[]api.ServiceResponse); ok {
		r0 = rf(ctx)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*[]api.ServiceResponse)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context) error); ok {
		r1 = rf(ctx)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// StopInstance provides a mock function with given fields: ctx, InstanceID
func (_m *Client) StopInstance(ctx context.Context, InstanceID string) error {
	ret := _m.Called(ctx, InstanceID)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string) error); ok {
		r0 = rf(ctx, InstanceID)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

type mockConstructorTestingTNewClient interface {
	mock.TestingT
	Cleanup(func())
}

// NewClient creates a new instance of Client. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewClient(t mockConstructorTestingTNewClient) *Client {
	mock := &Client{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
