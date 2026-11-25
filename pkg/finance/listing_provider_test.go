package finance

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/stretchr/testify/require"
)

func TestListingProviderUsesYahooForNonAlpacaProfiles(t *testing.T) {
	path := filepath.Join(t.TempDir(), "profiles.yaml")
	body := "profiles:\n" +
		"  - name: saxo-paper\n    provider: saxo\n" +
		"  - name: ig-paper\n    provider: ig\n" +
		"  - name: ibkr-paper\n    provider: ibkr\n"
	require.NoError(t, os.WriteFile(path, []byte(body), 0o600))
	t.Setenv(environment.TradingProfilesFile, path)

	for _, name := range []string{"saxo-paper", "ig-paper", "ibkr-paper"} {
		got, err := ListingProvider(name)
		require.NoError(t, err)
		require.Equal(t, "yahoo", got)
	}
}
