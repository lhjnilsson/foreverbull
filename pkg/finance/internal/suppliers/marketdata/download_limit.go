package marketdata

// MaxDownloadSymbols is how many distinct symbols one price download request
// may carry. The limit is per request, not per backtest run or per user:
// backtest create sends one request for its whole universe, so a backtest
// can't have more symbols than this. Symbols are counted by
// DownloadSymbolCount.
const MaxDownloadSymbols = 10_000

// DownloadSymbolCount is how many distinct symbols a download request
// carries, the number MaxDownloadSymbols limits. Symbols are compared as
// typed, so a repeat counts once.
//
// Switches to StoredSymbol (BRK.B, BRK-B and BRKB as one) when
// lhjnilsson/foreverbull#223 lands.
func DownloadSymbolCount(symbols []string) int {
	seen := make(map[string]struct{}, len(symbols))
	for _, symbol := range symbols {
		seen[symbol] = struct{}{}
	}
	return len(seen)
}
