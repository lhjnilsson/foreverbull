package marketdata

import (
	"context"
	"errors"
	"fmt"
	"net"
)

// TimeoutError is a provider that didn't answer for a symbol in time: its
// request timed out or its fetch budget ran out. Error is the STATUS_ERROR
// text the backtest service matches, so its shape is fixed. The download's
// own time limit is a TimeLimitError instead.
type TimeoutError struct {
	Provider string
	Symbol   string
}

func (e *TimeoutError) Error() string {
	return fmt.Sprintf("%s took too long to respond for %s", ShownProvider(e.Provider), e.Symbol)
}

// TimeLimitError is a download stopped by its overall time limit, not by a
// slow provider. Symbol is the first symbol it hadn't answered for. Error is
// the DeadlineExceeded text the backtest service matches, so its shape is
// fixed.
type TimeLimitError struct {
	Symbol string
}

func (e *TimeLimitError) Error() string {
	return "the download hit its time limit at " + e.Symbol
}

// IsProviderTimeout reports an error from a provider call that ran out of
// time: a deadline on the call's ctx or a network timeout. The caller checks
// its own ctx first, because a cancelled download is not the provider's fault.
func IsProviderTimeout(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}
