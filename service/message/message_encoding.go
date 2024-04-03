package message

import (
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	"github.com/mitchellh/mapstructure"
	"github.com/shopspring/decimal"
)

/*
Encode
Encode the structure into a byte- array that can be sent over eg. socket
*/
func encode(data interface{}) ([]byte, error) {
	bytes, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("error encoding message: %w", err)
	}
	return bytes, nil
}

/*
Decode
Decodes Bytes into struct, based on JSON- fields.
*/
func decode(bytes []byte, data interface{}) error {
	err := json.Unmarshal(bytes, data)
	if err != nil {
		return fmt.Errorf("error decoding message: %w", err)
	}
	return nil
}

/*
DecodeData
Dynamic way to insert expected data- structure which the data should me decoded to
*/
func decodeData(input interface{}, result interface{}) error {
	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		Metadata: nil,
		DecodeHook: mapstructure.ComposeDecodeHookFunc(
			ToTimeHookFunc(),
			ToDecimalHookFunc(),
		),
		Result: result,
	})
	if err != nil {
		return err
	}

	return decoder.Decode(input)
}

func ToTimeHookFunc() mapstructure.DecodeHookFunc {
	return func(
		f reflect.Type,
		t reflect.Type,
		data interface{}) (interface{}, error) {
		if t != reflect.TypeOf(time.Time{}) {
			return data, nil
		}

		switch f.Kind() {
		case reflect.String:
			return time.Parse(time.RFC3339, data.(string))
		case reflect.Int64:
			return time.Unix(0, data.(int64)*int64(time.Millisecond)), nil
		default:
			return data, nil
		}
	}
}

func ToDecimalHookFunc() mapstructure.DecodeHookFunc {
	return func(
		f reflect.Type,
		t reflect.Type,
		data interface{}) (interface{}, error) {
		if t != reflect.TypeOf(decimal.Decimal{}) {
			return data, nil
		}
		fmt.Println("f.Kind()", f.Kind())

		switch f.Kind() {
		case reflect.String:
			return decimal.NewFromString(data.(string))
		case reflect.Float64:
			return decimal.NewFromFloat(data.(float64)), nil
		case reflect.Int:
			return decimal.NewFromInt(int64(data.(int))), nil
		default:
			return data, nil
		}
	}
}
