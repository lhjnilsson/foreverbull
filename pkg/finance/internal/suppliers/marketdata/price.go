package marketdata

import (
	"fmt"
	"strings"

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
		meta := yahoo
		if meta == nil {
			meta, err = NewYahooClient()
			if err != nil {
				return nil, err
			}
		}
		return NewStooqYahoo(func() (supplier.Marketdata, error) {
			return meta, nil
		})
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
