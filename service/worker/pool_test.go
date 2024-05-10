package worker

import (
	"testing"

	"github.com/lhjnilsson/foreverbull/service/socket"
	"github.com/lhjnilsson/foreverbull/tests/helper"
	mockSocket "github.com/lhjnilsson/foreverbull/tests/mocks/service/socket"
	"github.com/stretchr/testify/suite"
	_ "go.nanomsg.org/mangos/v3/transport/all"
)

type PoolTest struct {
	suite.Suite
	pool           *pool
	instanceSocket *mockSocket.ContextSocket
	poolSocket     *mockSocket.ContextSocket
	rw             *mockSocket.ReadWriter
}

func (test *PoolTest) SetupTest() {
	test.instanceSocket = mockSocket.NewContextSocket(test.T())
	test.poolSocket = mockSocket.NewContextSocket(test.T())

	test.rw = mockSocket.NewReadWriter(test.T())
	test.pool = &pool{
		Socket: &socket.Socket{},
		socket: test.poolSocket,
	}
	helper.SetupEnvironment(test.T(), &helper.Containers{})
}

func (test *PoolTest) TearDownTest() {

}

func TestPool(t *testing.T) {
	suite.Run(t, new(PoolTest))
}
