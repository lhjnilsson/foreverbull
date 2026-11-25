package finance

import (
	"context"

	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/internal/usercopy"
	"github.com/lhjnilsson/foreverbull/pkg/finance/internal/suppliers/marketdata"
	"github.com/lhjnilsson/foreverbull/pkg/finance/internal/suppliers/trading"
)

// Quote is what a provider reported for one symbol. Reported is false when
// the provider does not send an exchange or a currency.
type Quote = marketdata.Quote

// ErrUnknownSymbol matches a provider answer that the ticker has no quote.
// It is not a listing refusal and not a failed lookup.
var ErrUnknownSymbol = marketdata.ErrUnknownSymbol

// QuoteFailure is a listing lookup that did not answer.
type QuoteFailure = marketdata.QuoteFailure

// QuoteFailures is every listing lookup that did not answer.
type QuoteFailures = marketdata.QuoteFailures

// SetQuoteReader replaces the listing lookup. Tests use it.
func SetQuoteReader(fn func(ctx context.Context, provider, profile, symbol string) (Quote, error)) func() {
	if fn == nil {
		return marketdata.SetQuoteReader(nil)
	}
	return marketdata.SetQuoteReader(func(ctx context.Context, provider, profile, symbol string) (marketdata.Quote, error) {
		return fn(ctx, provider, profile, symbol)
	})
}

// RefuseListed returns every symbol listed outside the US or priced in a
// currency other than USD. benchmark is checked the same way and is not
// repeated in the member list.
func RefuseListed(ctx context.Context, provider, profile string, members []string, benchmark string) ([]usercopy.MarketSymbol, *usercopy.MarketSymbol, error) {
	return marketdata.RefuseListed(ctx, provider, profile, members, benchmark)
}

// ListingsAnswered reports whether every symbol was answered by a listing
// lookup in this process. An unknown symbol is not an answer. The flag that
// skips later checks is stored only when this is true.
func ListingsAnswered(provider string, symbols []string) bool {
	return marketdata.ListingsAnswered(provider, symbols)
}

// ErrProfilesFile is a profiles file that is missing or cannot be read.
var ErrProfilesFile = trading.ErrProfilesFile

// ErrProfileNotFound is a profile name that is not in the profiles file.
var ErrProfileNotFound = trading.ErrProfileNotFound

// ProfileProvider is the provider field a trading profile names.
// An empty name is yahoo. A missing file or an unknown name is an error.
func ProfileProvider(name string) (string, error) {
	return trading.ProfileProvider(environment.GetTradingProfilesFile(), name)
}

// ListingProvider is the price source that answers a deployment's listing
// check. An Alpaca profile is checked with Alpaca. Every other profile, and
// a deployment with no profile, is checked with Yahoo. The sentence names
// this source. A missing file or an unknown profile name is an error.
func ListingProvider(profileName string) (string, error) {
	named, err := ProfileProvider(profileName)
	if err != nil {
		return "", err
	}
	if named == "alpaca" {
		return "alpaca", nil
	}
	return "yahoo", nil
}

// Unanswered are the symbols a listing check did not get an answer for.
func Unanswered(provider string, symbols []string) []string {
	return marketdata.Unanswered(provider, symbols)
}
