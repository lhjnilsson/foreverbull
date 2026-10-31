package command

import (
	"errors"
	"strings"
	"testing"
	"time"

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

// Only the finance service's exact ambiguous-ticker sentence, for the symbol
// the response is about, names the provider and the ticker.
func TestAmbiguousSymbolProviderOnlyMatchesTheRefusal(t *testing.T) {
	for _, provider := range []string{"yahoo", "stooq", "alpaca"} {
		got, ok := ambiguousSymbolProvider(provider+" has more than one instrument for SYM", "SYM")
		require.True(t, ok, provider)
		require.Equal(t, provider, got)
	}
	got, ok := ambiguousSymbolProvider("yahoo has more than one instrument for BRK.B", "BRK.B")
	require.True(t, ok)
	require.Equal(t, "yahoo", got)

	for name, tc := range map[string]struct{ text, symbol string }{
		"unknown provider":    {"postgres has more than one instrument for SYM", "SYM"},
		"upper-case provider": {"YAHOO has more than one instrument for SYM", "SYM"},
		"other symbol":        {"yahoo has more than one instrument for OTHER", "SYM"},
		"escape in symbol":    {"yahoo has more than one instrument for S\x1b[2JM", "S\x1b[2JM"},
		"space in symbol":     {"yahoo has more than one instrument for S M", "S M"},
		"symbol too long":     {"yahoo has more than one instrument for ABCDEFGHIJKLMNOPQRSTUVWXYZABCDEFG", "ABCDEFGHIJKLMNOPQRSTUVWXYZABCDEFG"},
		"trailing raw text":   {"yahoo has more than one instrument for SYM: 2 rows", "SYM"},
		"trailing period":     {"yahoo has more than one instrument for SYM.", "SYM"},
		"leading raw text":    {"error: yahoo has more than one instrument for SYM", "SYM"},
		"repository text":     {"symbol SYM maps to 2 instruments at 2024-01-01", "SYM"},
		"empty":               {"", ""},
	} {
		_, ok := ambiguousSymbolProvider(tc.text, tc.symbol)
		require.False(t, ok, name)
	}
}

// Only the finance service's exact timeout sentence, for the symbol the
// response is about, names the provider and the ticker.
func TestProviderTimeoutProviderOnlyMatchesTheSentence(t *testing.T) {
	for _, provider := range []string{"yahoo", "stooq", "alpaca"} {
		got, ok := providerTimeoutProvider(provider+" took too long to respond for AAPL", "AAPL")
		require.True(t, ok, provider)
		require.Equal(t, provider, got)
	}
	for name, text := range map[string]string{
		"other symbol":      "alpaca took too long to respond for MSFT",
		"unknown provider":  "The price provider took too long to respond for AAPL",
		"trailing raw text": "alpaca took too long to respond for AAPL: dial tcp",
		"leading raw text":  "error: alpaca took too long to respond for AAPL",
		"empty":             "",
	} {
		_, ok := providerTimeoutProvider(text, "AAPL")
		require.False(t, ok, name)
	}

}

// Only the finance service's DeadlineExceeded time-limit text counts as the
// limit, and its limit is read back. A provider timeout or the client's own
// deadline doesn't count.
func TestDownloadTimeLimitOnlyMatchesTheLimit(t *testing.T) {
	limit, ok := downloadTimeLimit(status.Error(codes.DeadlineExceeded, "the download hit its time limit of 2h0m0s at VOD.L"))
	require.True(t, ok)
	require.Equal(t, 2*time.Hour, limit)
	for name, err := range map[string]error{
		"provider timeout": status.Error(codes.DeadlineExceeded, "stooq took too long to respond for VOD.L"),
		"other code":       status.Error(codes.Internal, "the download hit its time limit of 2h0m0s at VOD.L"),
		"client deadline":  status.Error(codes.DeadlineExceeded, "context deadline exceeded"),
		"rate limiter":     status.Error(codes.DeadlineExceeded, "rate: Wait(n=1) would exceed context deadline"),
		"not a status":     errors.New("the download hit its time limit of 2h0m0s at VOD.L"),
		"no limit":         status.Error(codes.DeadlineExceeded, "the download hit its time limit at VOD.L"),
		"zero limit":       status.Error(codes.DeadlineExceeded, "the download hit its time limit of 0s at VOD.L"),
		"symbol too long":  status.Error(codes.DeadlineExceeded, "the download hit its time limit of 2h0m0s at "+strings.Repeat("A", 33)),
		"trailing text":    status.Error(codes.DeadlineExceeded, "the download hit its time limit of 2h0m0s at VOD.L: boom"),
		"nil":              nil,
	} {
		_, ok := downloadTimeLimit(err)
		require.False(t, ok, name)
	}
}

// The time-limit sentence counts symbols like the progress line and names
// whole hours in hours and anything else in minutes, word for word. A
// backtest that couldn't be removed says so and says to delete it first.
func TestTimeLimitMessage(t *testing.T) {
	require.Equal(t, "Couldn't load data for backtest 'oef-2024q1': the download hit the 2-hour limit after "+
		"6,212 of 10,000 symbols. The backtest wasn't saved. Split the symbols across several backtests, "+
		"then run `fbull backtest create` again.", timeLimitMessage("oef-2024q1", 2*time.Hour, 6212, 10000, true))
	require.Equal(t, "Couldn't load data for backtest 'oef-2024q1': the download hit the 1-hour limit after "+
		"6,212 of 10,000 symbols. The backtest wasn't saved. Split the symbols across several backtests, "+
		"then run `fbull backtest create` again.", timeLimitMessage("oef-2024q1", time.Hour, 6212, 10000, true))
	require.Equal(t, "Couldn't load data for backtest 'oef-2024q1': the download hit the 90-minute limit after "+
		"6,212 of 10,000 symbols. The backtest wasn't saved. Split the symbols across several backtests, "+
		"then run `fbull backtest create` again.", timeLimitMessage("oef-2024q1", 90*time.Minute, 6212, 10000, true))
	require.Equal(t, "Couldn't load data for backtest 'oef-2024q1': the download hit the 2-hour limit after "+
		"6,212 of 10,000 symbols. The backtest was partly saved and couldn't be removed. Run "+
		"`fbull backtest delete oef-2024q1` first. Split the symbols across several backtests, "+
		"then run `fbull backtest create` again.", timeLimitMessage("oef-2024q1", 2*time.Hour, 6212, 10000, false))
}
