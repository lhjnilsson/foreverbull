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
