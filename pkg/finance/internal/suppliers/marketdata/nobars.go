package marketdata

import "errors"

// ErrNoBars matches a GetOHLC error that is the provider's answer, not a
// failure: it served the request and has no bars for the symbol in the
// range (Yahoo "Not Found", Stooq "No data" or a CSV with no rows in it).
// Any other GetOHLC error (HTTP 5xx, network, timeout, unreadable body) is a
// failure, and a download must not read it as "no prices".
var ErrNoBars = errors.New("provider has no bars for the symbol")

// noBarsError keeps the provider's own text and matches ErrNoBars.
type noBarsError struct{ text string }

func (e *noBarsError) Error() string { return e.text }

func (e *noBarsError) Is(target error) bool { return target == ErrNoBars }

func noBars(text string) error { return &noBarsError{text: text} }
