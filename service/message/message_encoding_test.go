package message

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
)

type WithTime struct {
	Time time.Time `mapstructure:"time"`
}

func TestDecodeTimes(t *testing.T) {
	t.Run("decode string", func(t *testing.T) {
		data := map[string]string{"time": "2022-01-29T21:04:38+00:00"}

		toDecode := WithTime{}
		err := decodeData(data, &toDecode)
		assert.Nil(t, err)
	})
	t.Run("decode int", func(t *testing.T) {
		data := map[string]int64{"time": 1643486678}

		toDecode := WithTime{}
		err := decodeData(data, &toDecode)
		assert.Nil(t, err)
	})
	t.Run("bad value", func(t *testing.T) {
		data := map[string]string{"time": "quater past eight"}

		toDecode := WithTime{}
		err := decodeData(data, &toDecode)
		assert.NotNil(t, err)
	})
}

type WithDecimal struct {
	Value decimal.Decimal `mapstructure:"value"`
}

func TestDecodeDecimals(t *testing.T) {
	t.Run("decode string", func(t *testing.T) {
		data := map[string]string{"value": "123.456"}

		toDecode := WithDecimal{}
		err := decodeData(data, &toDecode)
		assert.Nil(t, err)
	})
	t.Run("decode float", func(t *testing.T) {
		data := map[string]float64{"value": 123.456}

		toDecode := WithDecimal{}
		err := decodeData(data, &toDecode)
		assert.Nil(t, err)
	})
	t.Run("decode int", func(t *testing.T) {
		data := map[string]int{"value": 123}

		toDecode := WithDecimal{}
		err := decodeData(data, &toDecode)
		assert.Nil(t, err)
	})
	t.Run("bad value", func(t *testing.T) {
		data := map[string]string{"value": "quater past eight"}

		toDecode := WithDecimal{}
		err := decodeData(data, &toDecode)
		assert.NotNil(t, err)
	})
}
