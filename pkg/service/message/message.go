package message

import (
	fbError "github.com/lhjnilsson/foreverbull/internal/error"
)

const (
	ErrTaskNotInMessage = fbError.Error("task field not found in response")
	ErrDecode           = fbError.Error("unable to decode response")
	ErrDecodeData       = fbError.Error("unable to decode response data")
	ErrEncode           = fbError.Error("unable to encode request")
)
