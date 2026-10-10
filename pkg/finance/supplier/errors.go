package supplier

import "errors"

// Trading sentinels. Wrap with %w so errors.Is works.
var (
	// ErrNotTradable is an unknown ISIN, an empty broker id, or an empty ISIN at the adapter.
	ErrNotTradable = errors.New("instrument is not tradable")

	// ErrUnsized is an IG share delta that is not an integer number of deal units.
	ErrUnsized = errors.New("share delta is not a whole number of deal units")

	// ErrRejected is a local reject or a broker refusal. PlaceOrder returns (nil, ErrRejected).
	ErrRejected = errors.New("order rejected")

	// ErrUnmappedPosition is a live position that cannot be mapped to an ISIN.
	ErrUnmappedPosition = errors.New("position is not mapped to an ISIN")

	// ErrCurrency is an account currency other than TRADING_CURRENCY.
	ErrCurrency = errors.New("account currency mismatch")

	// ErrUnavailable is a network failure, HTTP 401, or HTTP 5xx. Do not retry.
	ErrUnavailable = errors.New("trading backend unavailable")
)
