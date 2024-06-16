package message

import (
	"errors"

	"github.com/lhjnilsson/foreverbull/pkg/service/socket"
)

/*
Request
Baseline for requests being sent and recieved.
*/
type Request struct {
	Task string      `json:"task"`
	Data interface{} `json:"data"`
}

func (r *Request) Encode() ([]byte, error) {
	return encode(r)
}

func (r *Request) Decode(bytes []byte) error {
	err := decode(bytes, r)
	if err != nil {
		return err
	}
	if len(r.Task) == 0 {
		return ErrTaskNotInMessage
	}
	return nil
}

func (r *Request) DecodeData(output interface{}) error {
	return decodeData(r.Data, output)
}

func (r *Request) Process(socket socket.ReadWriter) (*Response, error) {
	rsp := Response{}

	data, err := r.Encode()
	if err != nil {
		return nil, err
	}

	err = socket.Write(data)
	if err != nil {
		return nil, err
	}

	rspData, err := socket.Read()
	if err != nil {
		return nil, err
	}

	err = rsp.Decode(rspData)
	if err != nil {
		return nil, err
	}

	if rsp.HasError() {
		return nil, errors.New(rsp.Error)
	}
	return &rsp, nil
}
