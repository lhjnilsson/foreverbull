// Code generated by mockery v2.46.3. DO NOT EDIT.

package storage

import (
	context "context"

	minio "github.com/minio/minio-go/v7"
	mock "github.com/stretchr/testify/mock"
)

// MockStorage is an autogenerated mock type for the Storage type
type MockStorage struct {
	mock.Mock
}

// CreateObject provides a mock function with given fields: ctx, bucket, name, opts
func (_m *MockStorage) CreateObject(ctx context.Context, bucket Bucket, name string, opts ...func(*minio.PutObjectOptions) error) (*Object, error) {
	_va := make([]interface{}, len(opts))
	for _i := range opts {
		_va[_i] = opts[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, ctx, bucket, name)
	_ca = append(_ca, _va...)
	ret := _m.Called(_ca...)

	if len(ret) == 0 {
		panic("no return value specified for CreateObject")
	}

	var r0 *Object
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, Bucket, string, ...func(*minio.PutObjectOptions) error) (*Object, error)); ok {
		return rf(ctx, bucket, name, opts...)
	}
	if rf, ok := ret.Get(0).(func(context.Context, Bucket, string, ...func(*minio.PutObjectOptions) error) *Object); ok {
		r0 = rf(ctx, bucket, name, opts...)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*Object)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, Bucket, string, ...func(*minio.PutObjectOptions) error) error); ok {
		r1 = rf(ctx, bucket, name, opts...)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetObject provides a mock function with given fields: ctx, bucket, name
func (_m *MockStorage) GetObject(ctx context.Context, bucket Bucket, name string) (*Object, error) {
	ret := _m.Called(ctx, bucket, name)

	if len(ret) == 0 {
		panic("no return value specified for GetObject")
	}

	var r0 *Object
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, Bucket, string) (*Object, error)); ok {
		return rf(ctx, bucket, name)
	}
	if rf, ok := ret.Get(0).(func(context.Context, Bucket, string) *Object); ok {
		r0 = rf(ctx, bucket, name)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*Object)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, Bucket, string) error); ok {
		r1 = rf(ctx, bucket, name)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ListObjects provides a mock function with given fields: ctx, bucket
func (_m *MockStorage) ListObjects(ctx context.Context, bucket Bucket) (*[]Object, error) {
	ret := _m.Called(ctx, bucket)

	if len(ret) == 0 {
		panic("no return value specified for ListObjects")
	}

	var r0 *[]Object
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, Bucket) (*[]Object, error)); ok {
		return rf(ctx, bucket)
	}
	if rf, ok := ret.Get(0).(func(context.Context, Bucket) *[]Object); ok {
		r0 = rf(ctx, bucket)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*[]Object)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, Bucket) error); ok {
		r1 = rf(ctx, bucket)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// NewMockStorage creates a new instance of MockStorage. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewMockStorage(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockStorage {
	mock := &MockStorage{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
