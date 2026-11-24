package api

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUnknownSymbolMessage(t *testing.T) {
	t.Parallel()

	require.Equal(t, "unknown symbol FB", UnknownSymbolMessage("FB"))
	require.True(t, IsUnknownSymbol(UnknownSymbolMessage("FB")))
	require.False(t, IsUnknownSymbol("no results found"))
	require.False(t, IsUnknownSymbol("Quote not found for symbol: FB"))
	require.False(t, IsUnknownSymbol("Something went wrong. unknown symbol FB"))
}
