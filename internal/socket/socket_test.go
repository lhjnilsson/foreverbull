package socket

import (
	"errors"
	"testing"

	"github.com/lhjnilsson/foreverbull/internal/test_helper"
	common_pb "github.com/lhjnilsson/foreverbull/pkg/pb"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.nanomsg.org/mangos/v3"
	"google.golang.org/protobuf/proto"
)

func TestSockError(t *testing.T) {
	t.Parallel()

	require.ErrorIs(t, sockError(mangos.ErrClosed), ErrClosed)
	require.ErrorIs(t, sockError(mangos.ErrRecvTimeout), ErrReadTimeout)
	require.ErrorIs(t, sockError(mangos.ErrSendTimeout), ErrSendTimeout)

	wrapped := sockError(errors.New("boom"))
	require.ErrorContains(t, wrapped, "socket error")
	require.ErrorContains(t, wrapped, "boom")
}

type SocketTest struct {
	suite.Suite
}

func (test *SocketTest) SetupSuite() {
	test_helper.SetupEnv(test.T())
}

func TestSocket(t *testing.T) {
	suite.Run(t, new(SocketTest))
}

func (test *SocketTest) TestRequesterReplier() {
	replier, err := NewReplier("0.0.0.0", 5555, false)
	test.Require().NoError(err, "failed to create replier")

	requester, err := NewRequester("127.0.0.1", 5555, true)
	test.Require().NoError(err, "failed to create requester")

	type testCase struct {
		Task string
		Data []byte
	}

	testCases := []testCase{
		{
			Task: "test",
			Data: []byte("test"),
		},
		{
			Task: "test_no_data",
		},
	}

	for _, testCase := range testCases {
		test.Run(testCase.Task, func() {
			go func() {
				request := common_pb.Request{Task: testCase.Task, Data: testCase.Data}
				sock, err := replier.Receive(&request)
				test.NoError(err, "failed to receive")

				response := common_pb.Response{Task: request.Task, Data: request.Data}
				err = sock.Reply(&response)
				test.NoError(err, "failed to send")
			}()

			request := common_pb.Request{Task: testCase.Task}
			data, err := proto.Marshal(&request)
			test.Require().NoError(err, "failed to marshal data")

			request.Data = data
			response := common_pb.Response{}
			err = requester.Request(&request, &response)
			test.Require().NoError(err, "failed to request")
			test.Equal(request.Task, response.Task, "task mismatch")
			test.Equal(request.Data, response.Data, "data mismatch")
		})
	}

	test.NoError(replier.Close(), "failed to close replier")
	test.NoError(requester.Close(), "failed to close requester")
}

func (test *SocketTest) TestListenToFreePort() {
	test.Run("requester", func() {
		requester, err := NewRequester("0.0.0.0", 0, false)
		test.Require().NoError(err, "failed to create requester")
		test.NotEqual(0, requester.GetPort(), "port is 0")
		test.NoError(requester.Close(), "failed to close requester")
	})
	test.Run("replier", func() {
		replier, err := NewReplier("0.0.0.0", 0, false)
		test.Require().NoError(err, "failed to create replier")
		test.NotEqual(0, replier.GetPort(), "port is 0")
		test.NoError(replier.Close(), "failed to close replier")
	})
}
