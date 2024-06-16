package message

/*
Response
Baseline for responses being sent and retrieved over socket.
In case there has been an error during processing we recieve an error- message
*/
type Response struct {
	Task  string      `json:"task"`
	Error string      `json:"error"`
	Data  interface{} `json:"data"`
}

func (r *Response) HasError() bool {
	return len(r.Error) > 0
}

func (r *Response) Encode() ([]byte, error) {
	return encode(r)
}

func (r *Response) Decode(bytes []byte) error {
	err := decode(bytes, r)
	if err != nil {
		return err
	}
	if len(r.Task) == 0 {
		return ErrTaskNotInMessage
	}
	return nil
}

func (r *Response) DecodeData(output interface{}) error {
	return decodeData(r.Data, output)
}
