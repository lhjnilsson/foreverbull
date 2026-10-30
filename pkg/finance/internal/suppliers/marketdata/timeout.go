package marketdata

import (
	"context"
	"errors"
	"fmt"
	"net"
)

// TimeoutError is a provider that didn't answer for a symbol in time: its
// request timed out, its fetch budget ran out, or the download's time limit
// ended while the symbol wasn't stored yet. Error is the STATUS_ERROR text
// the backtest service matches, so its shape is fixed.
type TimeoutError struct {
	Provider string
	Symbol   string
}

func (e *TimeoutError) Error() string {
	return fmt.Sprintf("%s took too long to respond for %s", ShownProvider(e.Provider), e.Symbol)
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
