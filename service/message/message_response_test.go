package message

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

type BadResponse struct {
	Uppdrag string `json:"uppdrag"`
}

type RandomData struct {
	Key string `json:"key"`
}

func TestResponse(t *testing.T) {
	t.Run("decode normal", func(t *testing.T) {
		sample := Response{Task: "hello"}
		sample_bytes, _ := json.Marshal(sample)

		rsp := Response{}
		err := rsp.Decode(sample_bytes)
		assert.Nil(t, err)
	})
	t.Run("decode expect decoding issue", func(t *testing.T) {
		sample_bytes := []byte("lorem imsum")
		rsp := Response{}
		err := rsp.Decode(sample_bytes)
		assert.NotNil(t, err)
	})
	t.Run("decode task not in response", func(t *testing.T) {
		sample := BadResponse{Uppdrag: "hello"}
		sample_bytes, _ := json.Marshal(sample)

		rsp := Response{}
		err := rsp.Decode(sample_bytes)
		assert.NotNil(t, err)
	})
	t.Run("decode data normal", func(t *testing.T) {
		normaldata := map[string]string{"hello": "people"}
		sample := Response{Task: "hello", Data: normaldata}
		sample_bytes, _ := json.Marshal(sample)
		rsp := Response{}
		err := rsp.Decode(sample_bytes)
		assert.Nil(t, err)

		data := RandomData{Key: "value"}
		err = rsp.DecodeData(&data)
		assert.Nil(t, err)
	})
}
