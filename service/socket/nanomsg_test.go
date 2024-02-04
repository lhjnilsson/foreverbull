package socket

import (
	"fmt"
	"testing"
	"time"

	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/stretchr/testify/suite"
	"go.nanomsg.org/mangos/v3"
	"go.nanomsg.org/mangos/v3/protocol/rep"
	"go.nanomsg.org/mangos/v3/protocol/req"
)

type NanomsgTestSuite struct {
	suite.Suite
}

func (test *NanomsgTestSuite) SetupTest() {
	_ = environment.Setup()
}

func (test *NanomsgTestSuite) TearDownTest() {
}

func TestNanomsgSuite(t *testing.T) {
	suite.Run(t, new(NanomsgTestSuite))
}

func (test *NanomsgTestSuite) GetRequester(port int, dial bool) (mangos.Socket, func()) {
	sock, _ := req.NewSocket()
	test.NoError(sock.SetOption(mangos.OptionRecvDeadline, time.Second))
	test.NoError(sock.SetOption(mangos.OptionSendDeadline, time.Second))

	url := fmt.Sprintf("tcp://localhost:%d", port)
	if dial {
		test.NoError(sock.Dial(url))
	} else {
		test.NoError(sock.Listen(url))
	}
	return sock, func() { sock.Close() }
}

func (test *NanomsgTestSuite) GetReplier(port int, dial bool) (mangos.Socket, func()) {
	sock, _ := rep.NewSocket()
	test.Require().NoError(sock.SetOption(mangos.OptionRecvDeadline, time.Second))
	test.Require().NoError(sock.SetOption(mangos.OptionSendDeadline, time.Second))

	url := fmt.Sprintf("tcp://localhost:%d", port)
	if dial {
		test.NoError(sock.Dial(url))
	} else {
		test.NoError(sock.Listen(url))
	}
	return sock, func() { sock.Close() }
}

func (test *NanomsgTestSuite) TestConnect() {
	test.Run("dial", func() {
		_, closePeer := test.GetReplier(1337, false)
		defer closePeer()

		socket := NanomsgSocket{SocketType: Requester, Dial: true, Port: 1337}
		err := socket.Connect()
		test.Nil(err)
		socket.Close()
	})
	test.Run("listen", func() {
		socket := NanomsgSocket{SocketType: Replier, Listen: true, Port: 1337}
		err := socket.Connect()
		test.Nil(err)
		socket.Close()
	})
	test.Run("connectionUnkown", func() {
		socket := NanomsgSocket{SocketType: Replier, Port: 1337}
		err := socket.Connect()
		test.NotNil(err)
		socket.Close()
	})
	test.Run("unableToConnect", func() {
		socket := NanomsgSocket{SocketType: Replier, Dial: true, Port: 1337}
		err := socket.Connect()
		test.NotNil(err)
		socket.Close()
	})
	test.Run("free port", func() {
		socket := NanomsgSocket{SocketType: Replier, Listen: true, Port: 0}
		err := socket.Connect()
		test.Nil(err)
		socket.Close()
	})
}

func (test *NanomsgTestSuite) TestSocketTypes() {
	test.Run("publisher", func() {
		socket := NanomsgSocket{SocketType: Publisher, Listen: true, Port: 1337}
		err := socket.Connect()
		defer socket.Close()
		test.Nil(err)
		test.Equal("pub", socket.socket.Info().SelfName)
	})
	test.Run("subscriber", func() {
		socket := NanomsgSocket{SocketType: Subscriber, Listen: true, Port: 1337}
		err := socket.Connect()
		defer socket.Close()
		test.Nil(err)
		test.Equal("sub", socket.socket.Info().SelfName)
	})
	test.Run("requester", func() {
		socket := NanomsgSocket{SocketType: Requester, Listen: true, Port: 1337}
		err := socket.Connect()
		defer socket.Close()
		test.Nil(err)
		test.Equal("req", socket.socket.Info().SelfName)
	})
	test.Run("replier", func() {
		socket := NanomsgSocket{SocketType: Replier, Listen: true, Port: 1337}
		err := socket.Connect()
		defer socket.Close()
		test.Nil(err)
		test.Equal("rep", socket.socket.Info().SelfName)
	})
}

func (test *NanomsgTestSuite) TestRead() {
	test.Run("read", func() {
		peer, closePeer := test.GetRequester(1337, false)
		defer closePeer()

		socket := NanomsgSocket{SocketType: Replier, Dial: true, Port: 1337}
		err := socket.Connect()
		test.NoError(err)
		defer socket.Close()

		ctx, err := socket.Get()
		test.Nil(err)

		err = peer.Send([]byte("hello"))
		test.Nil(err)

		msg, err := ctx.Read()
		test.Nil(err)
		test.Equal([]byte("hello"), msg)
	})
	test.Run("readTimeout", func() {
		socket := NanomsgSocket{SocketType: Replier, Listen: true, Port: 1337, RecvTimeout: 1}
		err := socket.Connect()
		defer socket.Close()
		test.Nil(err)

		ctx, err := socket.Get()
		test.Nil(err)

		_, err = ctx.Read()
		test.NotNil(err)
	})
	test.Run("socketClosed", func() {
		socket := NanomsgSocket{SocketType: Replier, Listen: true, Port: 1337, RecvTimeout: 1}
		err := socket.Connect()
		test.Nil(err)
		err = socket.Close()
		test.Nil(err)

		_, err = socket.Get()
		test.NotNil(err)
	})
}

func (test *NanomsgTestSuite) TestWrite() {
	test.Run("write", func() {
		_, closePeer := test.GetReplier(1337, false)
		defer closePeer()

		socket := NanomsgSocket{SocketType: Requester, Dial: true, Port: 1337, SEndout: 1}
		err := socket.Connect()
		defer socket.Close()
		test.Nil(err)

		ctx, err := socket.Get()
		test.Nil(err)

		err = ctx.Write([]byte("hello"))
		test.Nil(err)
	})
	test.Run("writeClosed", func() {
		_, closePeer := test.GetReplier(1337, false)
		defer closePeer()

		socket := NanomsgSocket{SocketType: Requester, Dial: true, Port: 1337, SEndout: 1}
		err := socket.Connect()
		socket.Close()
		test.Nil(err)

		_, err = socket.Get()
		test.NotNil(err)
	})
}

func (test *NanomsgTestSuite) TestClose() {
	test.Run("close", func() {
		socket := NanomsgSocket{SocketType: Requester, Listen: true, Port: 1337}
		err := socket.Connect()
		test.Nil(err)
		err = socket.Close()
		test.Nil(err)
	})
	test.Run("SocketClosed", func() {
		socket := NanomsgSocket{SocketType: Requester, Listen: true, Port: 1337}
		err := socket.Connect()
		test.Nil(err)
		err = socket.Close()
		test.Nil(err)

		err = socket.Close()
		test.NotNil(err)
	})
}

func (test *NanomsgTestSuite) TestContext() {
	test.Run("open close context", func() {
		socket := NanomsgSocket{SocketType: Requester, Listen: true, Port: 1337}
		err := socket.Connect()
		test.Nil(err)
		defer socket.Close()

		ctx, err := socket.Get()
		test.Nil(err)

		err = ctx.Close()
		test.Nil(err)
	})
}
