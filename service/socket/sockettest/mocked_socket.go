package sockettest

import (
	"github.com/lhjnilsson/foreverbull/service/socket"
	"github.com/stretchr/testify/mock"
)

type MockedSocket struct {
	mock.Mock
}

func (m *MockedSocket) Read() ([]byte, error) {
	args := m.Called()
	return args.Get(0).([]byte), args.Error(1)
}

func (m *MockedSocket) Write(data []byte) error {
	args := m.Called(data)

	return args.Error(0)
}

func (m *MockedSocket) Close() error {
	args := m.Called()

	return args.Error(0)
}

type MockedContextSocket struct {
	mock.Mock
}

func (m *MockedContextSocket) Get() (socket.ReadWriter, error) {
	args := m.Called()
	return args.Get(0).(socket.ReadWriter), args.Error(1)
}

func (m *MockedContextSocket) Close() error {
	args := m.Called()

	return args.Error(0)
}
