package message

import (
	"errors"
	"testing"

	"github.com/lhjnilsson/foreverbull/pkg/service/socket/sockettest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestRequest(t *testing.T) {
	t.Run("encode normal", func(t *testing.T) {
		r := Request{Task: "example"}
		_, err := r.Encode()
		assert.Nil(t, err)
	})
	t.Run("encode error request", func(t *testing.T) {
		r := Request{Task: "example", Data: make(chan int)}
		_, err := r.Encode()
		assert.NotNil(t, err)
	})
	t.Run("decode error", func(t *testing.T) {
		data := []byte("hello this is bad")

		r := Request{}
		err := r.Decode(data)
		assert.NotNil(t, err)
	})
	t.Run("decode TaskNotInMessage", func(t *testing.T) {
		toData := Request{}
		data, err := toData.Encode()
		assert.Nil(t, err)

		r := Request{}
		err = r.Decode(data)
		assert.NotNil(t, err)
	})
	t.Run("Decode data error", func(t *testing.T) {
		r := Request{Data: []byte("this is not really nice")}
		rsp := Response{}

		err := r.DecodeData(&rsp)
		assert.NotNil(t, err)
	})
}

func TestReuqestProcess(t *testing.T) {
	t.Run("fail Write", func(t *testing.T) {
		mockedSocket := new(sockettest.MockedSocket)
		mockedSocket.On("Write", mock.Anything).Return(errors.New("cant write"))

		req := Request{Task: "something"}
		_, err := req.Process(mockedSocket)

		assert.NotNil(t, err)
		assert.Equal(t, "cant write", err.Error())
	})
	t.Run("fail read", func(t *testing.T) {
		mockedSocket := new(sockettest.MockedSocket)
		mockedSocket.On("Write", mock.Anything).Return(nil)
		mockedSocket.On("Read").Return([]byte{}, errors.New("cant read"))

		req := Request{Task: "something"}
		_, err := req.Process(mockedSocket)

		assert.NotNil(t, err)
		assert.Equal(t, "cant read", err.Error())
	})
	t.Run("fail decode", func(t *testing.T) {
		rspData := []byte("hello this is not as expected")
		mockedSocket := new(sockettest.MockedSocket)
		mockedSocket.On("Write", mock.Anything).Return(nil)
		mockedSocket.On("Read").Return(rspData, nil)

		req := Request{Task: "something"}
		_, err := req.Process(mockedSocket)

		assert.NotNil(t, err)
	})
	t.Run("response has error", func(t *testing.T) {
		rsp := Response{Task: "something", Error: "error in processing"}
		rspData, err := rsp.Encode()
		assert.Nil(t, err)

		mockedSocket := new(sockettest.MockedSocket)
		mockedSocket.On("Write", mock.Anything).Return(nil)
		mockedSocket.On("Read").Return(rspData, nil)

		req := Request{Task: "something"}
		_, err = req.Process(mockedSocket)

		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "error in processing")
	})
}
