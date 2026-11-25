// Package priceunit is the one rule for a listing's price unit.
// Finance's identity store and the trading listings file both use it.
package priceunit

import (
	"fmt"
	"strings"
)

// Canonical is the price unit that is stored.
//
// GBX is one hundredth of a pound, so a price in GBX is 100 times the same
// price in GBP. Only GBp, and GBX in any case, mean pence, and both are
// stored as GBX. Lowercase gbp means pounds: it is folded to GBP, the same
// as any other currency code. An empty unit stays empty and is not filled
// from the currency. Any other unit must be the currency itself, or GBX
// when the currency is GBP. A refusal quotes the unit as it was written.
func Canonical(currency, unit string) (string, error) {
	raw := strings.TrimSpace(unit)
	if raw == "" {
		return "", nil
	}
	var canonical string
	if raw == "GBp" || strings.EqualFold(raw, "GBX") {
		canonical = "GBX"
	} else {
		canonical = strings.ToUpper(raw)
	}
	if canonical == currency {
		return canonical, nil
	}
	if currency == "GBP" && canonical == "GBX" {
		return canonical, nil
	}
	if currency == "" {
		return "", fmt.Errorf("price unit '%s' is not a unit of a currency", raw)
	}
	return "", fmt.Errorf("price unit '%s' is not a unit of %s", raw, currency)
}
