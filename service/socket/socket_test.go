package socket

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.nanomsg.org/mangos/v3/protocol/pub"
)

func TestSubPub(t *testing.T) {
	pub, err := pub.NewSocket()
	assert.Nil(t, err)
	defer pub.Close()
	err = pub.Listen("tcp://127.0.0.1:1337")
	assert.Nil(t, err)

	socket := Socket{Type: Subscriber, Dial: true, Port: 1337}
	sub, err := GetSubscriberSocket(context.TODO(), &socket)
	assert.Nil(t, err)
	defer sub.Close()

	time.Sleep(time.Second) // Ugly, but we are sometimes not fast enough on connection

	assert.NoError(t, pub.Send([]byte("hello")))
	assert.NoError(t, pub.Send([]byte("world")))

	msg, err := sub.Read()
	assert.Nil(t, err)
	assert.Equal(t, []byte("hello"), msg)
	msg, err = sub.Read()
	assert.Nil(t, err)
	assert.Equal(t, []byte("world"), msg)
}
