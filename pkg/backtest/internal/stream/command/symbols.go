package command

import (
	"fmt"

	"github.com/lhjnilsson/foreverbull/pkg/finance"
	"github.com/lhjnilsson/foreverbull/pkg/pb/backtest"
)

// storeShareClassesOnce renames every spelling of a listed share class in a
// universe file to its stored name (BRK.B, BRK-B and BRKB are BRKB; VOD.L
// stays VOD.L), for the benchmark
// and each period's assets. Two spellings of one class in the same period
// are one asset; if they disagree on enabled the file is refused rather
// than guessed.
func storeShareClassesOnce(item *backtest.Backtest) error {
	item.Benchmark = finance.StoredSymbol(item.GetBenchmark())
	for _, period := range item.GetPeriods() {
		assets := make([]*backtest.Backtest_Period_Asset, 0, len(period.GetAssets()))
		byName := make(map[string]*backtest.Backtest_Period_Asset, len(period.GetAssets()))
		for _, asset := range period.GetAssets() {
			name := finance.StoredSymbol(asset.GetSymbol())
			if seen, ok := byName[name]; ok {
				if seen.GetEnabled() != asset.GetEnabled() {
					return fmt.Errorf("symbol %s is listed twice in one period with different enabled values (as %s)",
						name, asset.GetSymbol())
				}
				continue
			}
			asset.Symbol = name
			byName[name] = asset
			assets = append(assets, asset)
		}
		period.Assets = assets
	}
	return nil
}
