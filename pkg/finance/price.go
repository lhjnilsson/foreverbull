package finance

import (
	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/pkg/finance/internal/suppliers/marketdata"
)

// ValidatePriceSource checks a backtest price source before download.
// yahoo and stooq only check the name. alpaca resolves a profile from the
// profiles file and fails when that profile cannot be chosen.
// It does not construct a Yahoo client and does not download bars.
func ValidatePriceSource(provider, profile string) error {
	return marketdata.ValidatePriceSource(provider, profile, environment.GetTradingProfilesFile())
}

// StoredSymbol is the one name a listed share class is stored under, however
// a universe file spells it: BRK.B, BRK-B, BRK/B and BRKB all store as BRKB.
// Every other symbol, including an exchange suffix such as VOD.L, is
// returned exactly as typed.
func StoredSymbol(symbol string) string {
	return marketdata.StoredSymbol(symbol)
}
