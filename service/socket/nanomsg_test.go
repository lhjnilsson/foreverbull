package socket

import (
	"fmt"
	"testing"
	"time"

	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/stretchr/testify/assert"
	"go.nanomsg.org/mangos/v3"
	"go.nanomsg.org/mangos/v3/protocol/rep"
	"go.nanomsg.org/mangos/v3/protocol/req"
)

func GetRequester(t *testing.T, port int, dial bool) (mangos.Socket, func()) {
	t.Helper()
	sock, _ := req.NewSocket()
	assert.NoError(t, sock.SetOption(mangos.OptionRecvDeadline, time.Second))
	assert.NoError(t, sock.SetOption(mangos.OptionSendDeadline, time.Second))

	url := fmt.Sprintf("tcp://localhost:%d", port)
	if dial {
		assert.NoError(t, sock.Dial(url))
	} else {
		assert.NoError(t, sock.Listen(url))
	}
	return sock, func() { sock.Close() }
}

func GetReplier(t *testing.T, port int, dial bool) (mangos.Socket, func()) {
	t.Helper()
	sock, _ := rep.NewSocket()
	assert.NoError(t, sock.SetOption(mangos.OptionRecvDeadline, time.Second))
	assert.NoError(t, sock.SetOption(mangos.OptionSendDeadline, time.Second))

	url := fmt.Sprintf("tcp://localhost:%d", port)
	if dial {
		assert.NoError(t, sock.Dial(url))
	} else {
		assert.NoError(t, sock.Listen(url))
	}
	return sock, func() { sock.Close() }
}

func TestConnect(t *testing.T) {
	// To set start and end port range
	_ = environment.Setup()
	t.Run("dial", func(t *testing.T) {
		_, closePeer := GetReplier(t, 1337, false)
		defer closePeer()

		socket := NanomsgSocket{SocketType: Requester, Dial: true, Port: 1337}
		err := socket.Connect()
		assert.Nil(t, err)
		socket.Close()
	})
	t.Run("listen", func(t *testing.T) {
		socket := NanomsgSocket{SocketType: Replier, Listen: true, Port: 1337}
		err := socket.Connect()
		assert.Nil(t, err)
		socket.Close()
	})
	t.Run("connectionUnkown", func(t *testing.T) {
		socket := NanomsgSocket{SocketType: Replier, Port: 1337}
		err := socket.Connect()
		assert.NotNil(t, err)
		socket.Close()
	})
	t.Run("unableToConnect", func(t *testing.T) {
		socket := NanomsgSocket{SocketType: Replier, Dial: true, Port: 1337}
		err := socket.Connect()
		assert.NotNil(t, err)
		socket.Close()
	})
	t.Run("free port", func(t *testing.T) {
		socket := NanomsgSocket{SocketType: Replier, Listen: true, Port: 0}
		err := socket.Connect()
		assert.Nil(t, err)
		socket.Close()
	})
}

func TestSocketTypes(t *testing.T) {
	t.Run("publisher", func(t *testing.T) {
		socket := NanomsgSocket{SocketType: Publisher, Listen: true, Port: 1337}
		err := socket.Connect()
		defer socket.Close()
		assert.Nil(t, err)
		assert.Equal(t, "pub", socket.socket.Info().SelfName)
	})
	t.Run("subscriber", func(t *testing.T) {
		socket := NanomsgSocket{SocketType: Subscriber, Listen: true, Port: 1337}
		err := socket.Connect()
		defer socket.Close()
		assert.Nil(t, err)
		assert.Equal(t, "sub", socket.socket.Info().SelfName)
	})
	t.Run("requester", func(t *testing.T) {
		socket := NanomsgSocket{SocketType: Requester, Listen: true, Port: 1337}
		err := socket.Connect()
		defer socket.Close()
		assert.Nil(t, err)
		assert.Equal(t, "req", socket.socket.Info().SelfName)
	})
	t.Run("replier", func(t *testing.T) {
		socket := NanomsgSocket{SocketType: Replier, Listen: true, Port: 1337}
		err := socket.Connect()
		defer socket.Close()
		assert.Nil(t, err)
		assert.Equal(t, "rep", socket.socket.Info().SelfName)
	})
}

func TestRead(t *testing.T) {
	t.Run("read", func(t *testing.T) {
		peer, closePeer := GetRequester(t, 1337, false)
		defer closePeer()

		socket := NanomsgSocket{SocketType: Replier, Dial: true, Port: 1337}
		err := socket.Connect()
		assert.NoError(t, err)
		defer socket.Close()

		ctx, err := socket.Get()
		assert.Nil(t, err)

		err = peer.Send([]byte("hello"))
		assert.Nil(t, err)

		msg, err := ctx.Read()
		assert.Nil(t, err)
		assert.Equal(t, []byte("hello"), msg)
	})
	t.Run("readTimeout", func(t *testing.T) {
		socket := NanomsgSocket{SocketType: Replier, Listen: true, Port: 1337, RecvTimeout: 1}
		err := socket.Connect()
		defer socket.Close()
		assert.Nil(t, err)

		ctx, err := socket.Get()
		assert.Nil(t, err)

		_, err = ctx.Read()
		assert.NotNil(t, err)
	})
	t.Run("socketClosed", func(t *testing.T) {
		socket := NanomsgSocket{SocketType: Replier, Listen: true, Port: 1337, RecvTimeout: 1}
		err := socket.Connect()
		assert.Nil(t, err)
		err = socket.Close()
		assert.Nil(t, err)

		_, err = socket.Get()
		assert.NotNil(t, err)
	})
}

func TestWrite(t *testing.T) {
	t.Run("write", func(t *testing.T) {
		_, closePeer := GetReplier(t, 1337, false)
		defer closePeer()

		socket := NanomsgSocket{SocketType: Requester, Dial: true, Port: 1337, SEndout: 1}
		err := socket.Connect()
		defer socket.Close()
		assert.Nil(t, err)

		ctx, err := socket.Get()
		assert.Nil(t, err)

		err = ctx.Write([]byte("hello"))
		assert.Nil(t, err)
	})
	t.Run("writeClosed", func(t *testing.T) {
		_, closePeer := GetReplier(t, 1337, false)
		defer closePeer()

		socket := NanomsgSocket{SocketType: Requester, Dial: true, Port: 1337, SEndout: 1}
		err := socket.Connect()
		socket.Close()
		assert.Nil(t, err)

		_, err = socket.Get()
		assert.NotNil(t, err)
	})
}

func TestClose(t *testing.T) {
	t.Run("close", func(t *testing.T) {
		socket := NanomsgSocket{SocketType: Requester, Listen: true, Port: 1337}
		err := socket.Connect()
		assert.Nil(t, err)
		err = socket.Close()
		assert.Nil(t, err)
	})
	t.Run("SocketClosed", func(t *testing.T) {
		socket := NanomsgSocket{SocketType: Requester, Listen: true, Port: 1337}
		err := socket.Connect()
		assert.Nil(t, err)
		err = socket.Close()
		assert.Nil(t, err)

		err = socket.Close()
		assert.NotNil(t, err)
	})
}

func TestContext(t *testing.T) {
	t.Run("open close context", func(t *testing.T) {
		socket := NanomsgSocket{SocketType: Requester, Listen: true, Port: 1337}
		err := socket.Connect()
		assert.Nil(t, err)
		defer socket.Close()

		ctx, err := socket.Get()
		assert.Nil(t, err)

		err = ctx.Close()
		assert.Nil(t, err)
	})
}
