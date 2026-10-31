package finance

import "github.com/lhjnilsson/foreverbull/pkg/finance/internal/suppliers/marketdata"

// MaxDownloadSymbols is how many distinct symbols one price download request
// may carry (per request). Backtest create checks it before it stores a
// backtest, and the finance service refuses a larger request.
const MaxDownloadSymbols = marketdata.MaxDownloadSymbols

// DownloadSymbolCount counts symbols the way the finance service counts them
// against MaxDownloadSymbols: distinct StoredSymbol names, so a repeat and
// every spelling of one listed share class (BRK.B, BRK-B and BRKB) count
// once.
func DownloadSymbolCount(symbols []string) int {
	return marketdata.DownloadSymbolCount(symbols)
}
