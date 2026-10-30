package command

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const invalidTickerTail = " isn't a valid ticker (use 1 to 32 letters, digits, or . ^ = _ -)"

// Only the finance service's exact InvalidArgument refusal, for the count
// this create sent and a position inside it, is shown as an invalid ticker.
func TestInvalidTickerPositionOnlyMatchesTheRefusal(t *testing.T) {
	for _, tc := range []struct {
		message  string
		sent     int
		position int
	}{
		{"symbol 7 of 99" + invalidTickerTail, 99, 7},
		{"symbol 1 of 1" + invalidTickerTail, 1, 1},
		{"symbol 99 of 99" + invalidTickerTail, 99, 99},
	} {
		position, ok := invalidTickerPosition(status.Error(codes.InvalidArgument, tc.message), tc.sent)
		require.True(t, ok, tc.message)
		require.Equal(t, tc.position, position, tc.message)
	}

	for name, tc := range map[string]struct {
		err  error
		sent int
	}{
		"other code":        {status.Error(codes.Internal, "symbol 7 of 99"+invalidTickerTail), 99},
		"not a status":      {errors.New("symbol 7 of 99" + invalidTickerTail), 99},
		"count not sent":    {status.Error(codes.InvalidArgument, "symbol 7 of 98"+invalidTickerTail), 99},
		"position past end": {status.Error(codes.InvalidArgument, "symbol 100 of 99"+invalidTickerTail), 99},
		"position zero":     {status.Error(codes.InvalidArgument, "symbol 0 of 99"+invalidTickerTail), 99},
		"leading zero":      {status.Error(codes.InvalidArgument, "symbol 07 of 99"+invalidTickerTail), 99},
		"symbol echoed":     {status.Error(codes.InvalidArgument, "symbol AAPL of 99"+invalidTickerTail), 99},
		"trailing raw text": {status.Error(codes.InvalidArgument, "symbol 7 of 99"+invalidTickerTail+": <b>x</b>"), 99},
		"leading raw text":  {status.Error(codes.InvalidArgument, "error: symbol 7 of 99"+invalidTickerTail), 99},
		"old wording":       {status.Error(codes.InvalidArgument, "a symbol isn't a valid ticker (use 1 to 32 letters, digits, or . ^ = _ -)"), 99},
		"too many digits":   {status.Error(codes.InvalidArgument, "symbol 7 of 1234567"+invalidTickerTail), 1234567},
		"non-ascii digit":   {status.Error(codes.InvalidArgument, "symbol ٧ of 99"+invalidTickerTail), 99},
		"no detail":         {status.Error(codes.InvalidArgument, ""), 99},
		"nil":               {nil, 99},
	} {
		_, ok := invalidTickerPosition(tc.err, tc.sent)
		require.False(t, ok, name)
	}
}
