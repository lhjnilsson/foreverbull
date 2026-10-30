package marketdata

import "errors"

// ErrNoISIN matches a GetAsset error for an equity that the ISIN lookup
// answered has no ISIN: it served the request and does not list the ticker,
// or lists it with an empty ISIN. Without an ISIN the equity can't be
// followed across ticker changes. A failed ISIN request (HTTP error,
// network, unreadable body) is a failure and does not match.
var ErrNoISIN = errors.New("provider has no ISIN for the equity")

// noISINError keeps the provider's own text and matches ErrNoISIN.
type noISINError struct{ text string }

func (e *noISINError) Error() string { return e.text }

func (e *noISINError) Is(target error) bool { return target == ErrNoISIN }

func noISIN(text string) error { return &noISINError{text: text} }
