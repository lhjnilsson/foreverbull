package marketdata

// MaxDownloadSymbols is how many distinct symbols one price download request
// may carry. The limit is per request, not per backtest run or per user:
// backtest create sends one request for its whole universe, so a backtest
// can't have more symbols than this. Symbols are counted by
// DownloadSymbolCount.
const MaxDownloadSymbols = 10_000

// DownloadSymbolCount is how many distinct symbols a download request
// carries, the number MaxDownloadSymbols limits. Symbols are compared by
// StoredSymbol, so a repeat and every spelling of one listed share class
// (BRK.B, BRK-B and BRKB) count once.
func DownloadSymbolCount(symbols []string) int {
	seen := make(map[string]struct{}, len(symbols))
	for _, symbol := range symbols {
		seen[StoredSymbol(symbol)] = struct{}{}
	}
	return len(seen)
}
