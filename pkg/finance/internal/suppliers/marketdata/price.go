package marketdata

import (
	"errors"
	"fmt"
	"strings"

	"github.com/lhjnilsson/foreverbull/internal/usercopy"
	"github.com/lhjnilsson/foreverbull/pkg/finance/internal/suppliers/trading"
	"github.com/lhjnilsson/foreverbull/pkg/finance/supplier"
)

// NormalizeProvider accepts yahoo, stooq, and alpaca. Empty is supplier.DefaultProvider.
func NormalizeProvider(provider string) (string, error) {
	provider = strings.ToLower(strings.TrimSpace(provider))
	switch provider {
	case "":
		return supplier.DefaultProvider, nil
	case "yahoo", "stooq", "alpaca":
		return provider, nil
	default:
		return "", fmt.Errorf("unknown price provider %q; valid providers: yahoo, stooq, alpaca", provider)
	}
}

// RequireTradingProvider is the price source a trading session reads.
// Empty and unknown names fail. Yahoo is not substituted.
func RequireTradingProvider(provider string) (string, error) {
	if strings.TrimSpace(provider) == "" {
		return "", errors.New(usercopy.TradingProviderMissing(""))
	}
	name, err := NormalizeProvider(provider)
	if err != nil {
		return "", errors.New(usercopy.TradingProviderUnknown(provider, ""))
	}
	return name, nil
}

// RequireTradingPrice is RequireTradingProvider, and for stooq also the
// configured exchange. A missing exchange fails before a request is sent.
func RequireTradingPrice(provider string) (string, error) {
	name, err := RequireTradingProvider(provider)
	if err != nil {
		return "", err
	}
	if name != "stooq" {
		return name, nil
	}
	if _, err := ConfiguredStooqExchange(); err != nil {
		return "", err
	}
	return name, nil
}

// newStooqPriceClient builds the Stooq composite OHLCClient returns.
// A test replaces it to observe that client without calling Stooq.
var newStooqPriceClient = NewStooqYahoo

// FetchTradingClient is the price client a trading ingest calls.
// It uses OHLCClient, so yahoo and stooq are the same clients a download uses.
func FetchTradingClient(provider, profile, profilesFile string, yahoo supplier.Marketdata) (supplier.Marketdata, error) {
	name, err := RequireTradingProvider(provider)
	if err != nil {
		return nil, err
	}
	return OHLCClient(name, profile, profilesFile, yahoo)
}

// UnknownProviderName stands in for a provider ShownProvider can't name.
const UnknownProviderName = "The price provider"

// ShownProvider is the provider name for user-facing text: yahoo, stooq or
// alpaca from the NormalizeProvider allowlist, else UnknownProviderName. An
// empty name is unknown too, so the text never names a default it guessed.
func ShownProvider(provider string) string {
	if strings.TrimSpace(provider) == "" {
		return UnknownProviderName
	}
	name, err := NormalizeProvider(provider)
	if err != nil {
		return UnknownProviderName
	}
	return name
}

// ValidatePriceSource checks the name and, for alpaca, that a profile can be chosen.
// It does not construct a Yahoo client and does not download bars.
func ValidatePriceSource(provider, profile, profilesFile string) error {
	kind, err := NormalizeProvider(provider)
	if err != nil {
		return err
	}
	if kind != "alpaca" {
		return nil
	}
	_, _, err = trading.AlpacaPriceCredentials(profilesFile, profile)
	return err
}

// OHLCClient is the GetOHLC client for a backtest price source.
// yahoo uses the injected Yahoo client. stooq wraps that Yahoo for metadata.
// alpaca uses a profile key and fails before a client exists when the profile cannot be chosen.
func OHLCClient(provider, profile, profilesFile string, yahoo supplier.Marketdata) (supplier.Marketdata, error) {
	kind, err := NormalizeProvider(provider)
	if err != nil {
		return nil, err
	}
	switch kind {
	case "stooq":
		exchange, err := ConfiguredStooqExchange()
		if err != nil {
			return nil, err
		}
		meta := yahoo
		if meta == nil {
			meta, err = NewYahooClient()
			if err != nil {
				return nil, err
			}
		}
		return newStooqPriceClient(func() (supplier.Marketdata, error) {
			return meta, nil
		}, WithStooqExchange(exchange))
	case "alpaca":
		apiKey, apiSecret, err := trading.AlpacaPriceCredentials(profilesFile, profile)
		if err != nil {
			return nil, err
		}
		return NewAlpacaBars(apiKey, apiSecret)
	default:
		if yahoo == nil {
			return NewYahooClient()
		}
		return yahoo, nil
	}
}
