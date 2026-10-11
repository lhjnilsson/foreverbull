package trading

import (
	"context"
	"strings"
	"time"
)

// Directory maps a ticker alias to ISINs at a time. Ticker is not identity.
// Tests use MemoryDirectory. A database-backed directory reads instrument and
// asset_symbol when one is configured.
type Directory interface {
	// Resolve returns the distinct ISINs whose alias matches symbol at at.
	// Zero matches is an empty slice, not an error.
	Resolve(ctx context.Context, symbol string, at time.Time) ([]string, error)
}

// SymbolAlias is one half-open [From, To) ticker for an ISIN. A nil To is open.
type SymbolAlias struct {
	ISIN   string
	Symbol string
	From   time.Time
	To     *time.Time
}

// MemoryDirectory is an immutable in-memory alias book.
type MemoryDirectory struct {
	aliases []SymbolAlias
}

// NewMemoryDirectory copies aliases. The result does not share the caller's slice.
func NewMemoryDirectory(aliases ...SymbolAlias) *MemoryDirectory {
	copied := make([]SymbolAlias, len(aliases))
	copy(copied, aliases)
	return &MemoryDirectory{aliases: copied}
}

// Resolve implements Directory.
func (d *MemoryDirectory) Resolve(ctx context.Context, symbol string, at time.Time) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if d == nil {
		return nil, nil
	}
	symbol = strings.TrimSpace(symbol)
	if symbol == "" {
		return nil, nil
	}
	at = dateOnly(at)

	var out []string
	seen := make(map[string]struct{})
	for _, alias := range d.aliases {
		if strings.TrimSpace(alias.Symbol) != symbol {
			continue
		}
		if !aliasCovers(alias, at) {
			continue
		}
		isin := normalizeISIN(alias.ISIN)
		if isin == "" {
			continue
		}
		if _, ok := seen[isin]; ok {
			continue
		}
		seen[isin] = struct{}{}
		out = append(out, isin)
	}
	return out, nil
}

func aliasCovers(alias SymbolAlias, at time.Time) bool {
	if at.Before(dateOnly(alias.From)) {
		return false
	}
	if alias.To == nil {
		return true
	}
	return at.Before(dateOnly(*alias.To))
}

func dateOnly(t time.Time) time.Time {
	year, month, day := t.Date()
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}

func normalizeISIN(isin string) string {
	return strings.ToUpper(strings.TrimSpace(isin))
}
