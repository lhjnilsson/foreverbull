package marketdata

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDownloadSymbolCountCountsRepeatsOnce(t *testing.T) {
	t.Parallel()
	require.Equal(t, 2, DownloadSymbolCount([]string{"AAPL", "MSFT", "AAPL", "AAPL"}))
	require.Equal(t, 0, DownloadSymbolCount(nil))
}

// Every spelling of one listed share class is one symbol; an exchange suffix
// is not a share class.
func TestDownloadSymbolCountCountsShareClassSpellingsOnce(t *testing.T) {
	t.Parallel()
	require.Equal(t, 1, DownloadSymbolCount([]string{"BRK.B", "BRK-B", "BRKB"}))
	require.Equal(t, 3, DownloadSymbolCount([]string{"BRK.B", "AAPL", "BRKB", "VOD.L"}))
}
